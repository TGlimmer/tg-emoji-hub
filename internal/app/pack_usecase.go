package app

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

// shortNameRe 跟 crawler/lib/signals.py 的 _SHORT_NAME_RE 必须一致，改一边记得改另一边
var shortNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// AddManualResult AddManual 返回结果
//
// AlreadyExisted=true 时 Pack 是已有记录，handler 回 409 时仍要带上它的 pack_id
type AddManualResult struct {
	Pack           *entity.StickerPack
	AlreadyExisted bool
}

// PackUseCase 表情包用例
type PackUseCase struct {
	repo       port.PackRepository
	signalsDir string

	// telegramCheckURL 可替换，方便测试。nil 时用默认 t.me 地址
	telegramCheckURL func(shortName string) string
}

func NewPackUseCase(repo port.PackRepository, signalsDir string) *PackUseCase {
	return &PackUseCase{repo: repo, signalsDir: signalsDir}
}

// SetTelegramCheckURL 测试用：覆盖验证包是否存在时访问的 URL 生成函数
func (u *PackUseCase) SetTelegramCheckURL(fn func(shortName string) string) {
	u.telegramCheckURL = fn
}

// List 分页查询
func (u *PackUseCase) List(ctx context.Context, source string, page, pageSize int) ([]entity.StickerPack, int64, error) {
	source = strings.TrimSpace(source)
	if source != "" && source != "all" &&
		source != entity.PackSourceFeatured && source != entity.PackSourceManual {
		return nil, 0, apperr.ErrInvalidParam.WithMessage("source 非法，必须是 all/featured/manual")
	}
	return u.repo.List(ctx, port.PackFilter{
		Source:   source,
		Page:     page,
		PageSize: pageSize,
	})
}

// ListParams 扩展版列表参数（新端点选项）
type ListParams struct {
	Source      string
	Sort        string // newest / hot / name，默认 newest
	WithPreview bool
	PreviewSize int // 默认 4
	Page        int
	PageSize    int
}

// ListExtended 支持 sort + with_preview 的列表查询
//
// with_preview=true 时返回 []PackWithPreview，handler 渲染走 preview 路径；
// with_preview=false 时回退到旧的 List，preview 字段省略不返。
func (u *PackUseCase) ListExtended(ctx context.Context, p ListParams) ([]port.PackWithPreview, int64, error) {
	source := strings.TrimSpace(p.Source)
	if source != "" && source != "all" &&
		source != entity.PackSourceFeatured && source != entity.PackSourceManual {
		return nil, 0, apperr.ErrInvalidParam.WithMessage("source 非法，必须是 all/featured/manual")
	}
	sort := strings.TrimSpace(p.Sort)
	if sort != "" && !port.IsValidPackSort(sort) {
		return nil, 0, apperr.ErrInvalidParam.WithMessage("sort 非法，必须是 newest/hot/name")
	}

	filter := port.PackFilter{
		Source:      source,
		Sort:        port.NormalizePackSort(sort),
		WithPreview: p.WithPreview,
		PreviewSize: p.PreviewSize,
		Page:        p.Page,
		PageSize:    p.PageSize,
	}

	if p.WithPreview {
		return u.repo.ListWithPreview(ctx, filter)
	}
	// 不需要 preview → 用 List 包一层 PackWithPreview（Preview 字段为 nil）
	packs, total, err := u.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	out := make([]port.PackWithPreview, 0, len(packs))
	for i := range packs {
		out = append(out, port.PackWithPreview{Pack: packs[i]})
	}
	return out, total, nil
}

// AddManual 添加 manual 包并触发爬虫信号
//
// 步骤：
//  1. 验 short_name 正则
//  2. FindByShortName → 已存在返 AlreadyExisted=true（handler 转 409 + 现有 pack_id）
//  3. Create 新包（source=manual, last_crawled_at=NULL）
//  4. 写信号文件 signals_dir/{short_name}.pending；失败仅 wrap 不阻断（pack 已落库）
func (u *PackUseCase) AddManual(ctx context.Context, shortName string) (*AddManualResult, error) {
	shortName = strings.TrimSpace(shortName)
	if !shortNameRe.MatchString(shortName) {
		return nil, apperr.ErrInvalidParam.WithMessage(
			"short_name 必须匹配 ^[A-Za-z][A-Za-z0-9_]{0,63}$")
	}

	existing, err := u.repo.FindByShortName(ctx, shortName)
	if err != nil && !stderrors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return &AddManualResult{Pack: existing, AlreadyExisted: true}, nil
	}

	// Telegram 包名有效性 pre-check，网络错/超时不阻塞
	if err := u.verifyPackExistsOnTelegram(ctx, shortName); err != nil {
		return nil, err
	}

	pack := &entity.StickerPack{
		ShortName:  shortName,
		Title:      shortName,
		Source:     entity.NormalizeSource(entity.PackSourceManual),
		IsFeatured: false,
	}
	if err := u.repo.Create(ctx, pack); err != nil {
		// Create 失败若是 UNIQUE 冲突（并发场景）→ 再查一次返已存在
		if stderrors.Is(err, apperr.ErrConflict) {
			if got, e2 := u.repo.FindByShortName(ctx, shortName); e2 == nil && got != nil {
				return &AddManualResult{Pack: got, AlreadyExisted: true}, nil
			}
		}
		return nil, err
	}

	// 写信号文件触发 crawler；失败不阻断响应（pack 已落库可补救）
	if err := u.writeSignal(shortName); err != nil {
		return &AddManualResult{Pack: pack, AlreadyExisted: false},
			apperr.ErrInternal.WithMessage("pack 已创建，但信号文件写入失败").WithCause(err)
	}
	return &AddManualResult{Pack: pack, AlreadyExisted: false}, nil
}

// FindByShortName 单查
func (u *PackUseCase) FindByShortName(ctx context.Context, shortName string) (*entity.StickerPack, error) {
	shortName = strings.TrimSpace(shortName)
	if !shortNameRe.MatchString(shortName) {
		return nil, apperr.ErrInvalidParam.WithMessage("short_name 格式不合法")
	}
	return u.repo.FindByShortName(ctx, shortName)
}

// writeSignal 写 signals_dir/{short_name}.pending
//
// 必须在 short_name 通过正则后才能调用，这里再做一次 join 后 prefix 防御性检查
func (u *PackUseCase) writeSignal(shortName string) error {
	if u.signalsDir == "" {
		return stderrors.New("signals_dir 未配置")
	}
	if !shortNameRe.MatchString(shortName) {
		return stderrors.New("non-whitelisted short_name") // 双保险
	}
	if err := os.MkdirAll(u.signalsDir, 0o755); err != nil {
		return fmt.Errorf("mkdir signals_dir: %w", err)
	}

	filename := shortName + ".pending"
	full := filepath.Join(u.signalsDir, filename)
	// 防穿越：full 必须在 signals_dir 之下
	absDir, err := filepath.Abs(u.signalsDir)
	if err != nil {
		return fmt.Errorf("abs signals_dir: %w", err)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return fmt.Errorf("abs signal file: %w", err)
	}
	if !strings.HasPrefix(absFull, absDir+string(filepath.Separator)) && absFull != absDir {
		return stderrors.New("signal path escapes signals_dir")
	}

	// 内容格式与 crawler/lib/signals.py write_signal 一致
	body := fmt.Sprintf(`{"created_at":%d,"retry_count":0}`, time.Now().Unix())
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write signal: %w", err)
	}
	return nil
}

func (u *PackUseCase) verifyPackExistsOnTelegram(ctx context.Context, shortName string) error {
	url := "https://t.me/addemoji/" + shortName
	if u.telegramCheckURL != nil {
		url = u.telegramCheckURL(shortName)
	}
	return u.verifyWithURL(ctx, url)
}

// verifyWithURL 访问指定 URL 判断 emoji 包是否真实存在
//
// 不存在的包页面会 fallback 到 tg://addstickers（sticker 而非 emoji），
// 存在的包页面用 tg://addemoji。靠这个区分比解析文案更稳。
// 网络错误/超时一律放行，不因 TG 抖动拦用户。
func (u *PackUseCase) verifyWithURL(ctx context.Context, url string) error {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return nil
	}
	page := string(buf)

	if strings.Contains(page, "tg://addstickers") && !strings.Contains(page, "tg://addemoji") {
		return apperr.ErrInvalidParam.WithMessage(
			"该表情包在 Telegram 上不存在，请检查 short_name 是否正确")
	}

	return nil
}

package app

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/entity"
	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
	apperr "github.com/TGlimmer/tg-emoji-hub/pkg/errors"
)

type mockPackRepo struct {
	listFn   func(ctx context.Context, f port.PackFilter) ([]entity.StickerPack, int64, error)
	findByID func(ctx context.Context, id uint64) (*entity.StickerPack, error)
	findFn   func(ctx context.Context, shortName string) (*entity.StickerPack, error)
	createFn func(ctx context.Context, pack *entity.StickerPack) error
	lastList port.PackFilter
}

func (m *mockPackRepo) List(ctx context.Context, f port.PackFilter) ([]entity.StickerPack, int64, error) {
	m.lastList = f
	if m.listFn != nil {
		return m.listFn(ctx, f)
	}
	return nil, 0, nil
}

func (m *mockPackRepo) FindByShortName(ctx context.Context, shortName string) (*entity.StickerPack, error) {
	if m.findFn != nil {
		return m.findFn(ctx, shortName)
	}
	return nil, apperr.ErrNotFound
}

func (m *mockPackRepo) FindByID(ctx context.Context, id uint64) (*entity.StickerPack, error) {
	if m.findByID != nil {
		return m.findByID(ctx, id)
	}
	return nil, apperr.ErrNotFound
}

func (m *mockPackRepo) Create(ctx context.Context, pack *entity.StickerPack) error {
	if m.createFn != nil {
		return m.createFn(ctx, pack)
	}
	return nil
}

func (m *mockPackRepo) ListWithPreview(ctx context.Context, f port.PackFilter) ([]port.PackWithPreview, int64, error) {
	m.lastList = f
	packs, total, err := m.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out := make([]port.PackWithPreview, 0, len(packs))
	for i := range packs {
		out = append(out, port.PackWithPreview{Pack: packs[i]})
	}
	return out, total, nil
}

// skipTelegramCheck 返回一个始终"包存在"的 mock server 地址生成器
func skipTelegramCheck(t *testing.T) func(string) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<a href="tg://addemoji?set=Test">Add Emoji</a>`)
	}))
	t.Cleanup(ts.Close)
	return func(shortName string) string { return ts.URL + "/" + shortName }
}

// ---------------- List ----------------

func TestList_PassesFilter(t *testing.T) {
	repo := &mockPackRepo{}
	uc := NewPackUseCase(repo, "")
	_, _, _ = uc.List(context.Background(), "manual", 2, 50)
	if repo.lastList.Source != "manual" || repo.lastList.Page != 2 || repo.lastList.PageSize != 50 {
		t.Errorf("filter 透传错误: %+v", repo.lastList)
	}
}

func TestList_RejectsInvalidSource(t *testing.T) {
	uc := NewPackUseCase(&mockPackRepo{}, "")
	_, _, err := uc.List(context.Background(), "garbage", 1, 20)
	if !stderrors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("非法 source 应返 ErrInvalidParam, got %v", err)
	}
}

func TestList_AcceptsEmptyAndAll(t *testing.T) {
	uc := NewPackUseCase(&mockPackRepo{}, "")
	for _, s := range []string{"", "all", "featured", "manual"} {
		_, _, err := uc.List(context.Background(), s, 1, 20)
		if err != nil {
			t.Errorf("source=%q 应通过校验, got %v", s, err)
		}
	}
}

// ---------------- AddManual ----------------

func TestAddManual_RejectsInvalidShortName(t *testing.T) {
	uc := NewPackUseCase(&mockPackRepo{}, "")
	for _, bad := range []string{
		"",
		"1Pack",         // 数字开头
		"Pack!",         // 含 !
		"../etc/passwd", // 路径穿越
		"Pack Name",     // 含空格
		"a/b",           // 含 /
		"toolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolongtoolong", // > 64
	} {
		_, err := uc.AddManual(context.Background(), bad)
		if !stderrors.Is(err, apperr.ErrInvalidParam) {
			t.Errorf("short_name=%q 应被拒绝, got %v", bad, err)
		}
	}
}

func TestAddManual_ReturnsExistingWhenAlreadyExists(t *testing.T) {
	existing := &entity.StickerPack{ID: 42, ShortName: "Hi", Title: "Hi"}
	repo := &mockPackRepo{
		findFn: func(ctx context.Context, s string) (*entity.StickerPack, error) {
			return existing, nil
		},
	}
	uc := NewPackUseCase(repo, "")
	r, err := uc.AddManual(context.Background(), "Hi")
	if err != nil {
		t.Fatalf("不应出错: %v", err)
	}
	if !r.AlreadyExisted {
		t.Error("AlreadyExisted 应为 true")
	}
	if r.Pack.ID != 42 {
		t.Error("应返回现有 pack")
	}
}

func TestAddManual_CreatesAndWritesSignal(t *testing.T) {
	signalsDir := t.TempDir()
	var created *entity.StickerPack
	repo := &mockPackRepo{
		findFn: func(ctx context.Context, s string) (*entity.StickerPack, error) {
			return nil, apperr.ErrNotFound
		},
		createFn: func(ctx context.Context, pack *entity.StickerPack) error {
			pack.ID = 7
			created = pack
			return nil
		},
	}
	uc := NewPackUseCase(repo, signalsDir)
	uc.telegramCheckURL = skipTelegramCheck(t)

	r, err := uc.AddManual(context.Background(), "GoodPack")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.AlreadyExisted {
		t.Error("AlreadyExisted 应为 false")
	}
	if created == nil || created.Source != entity.PackSourceManual {
		t.Errorf("Create 应被调用, source 应为 manual, got %+v", created)
	}

	// 信号文件实际生成
	signalPath := filepath.Join(signalsDir, "GoodPack.pending")
	data, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("信号文件未生成: %v", err)
	}
	if !contains(string(data), `"retry_count":0`) {
		t.Errorf("信号文件内容异常: %s", data)
	}
}

func TestAddManual_SignalWriteFailureReturnsInternalButPackPersists(t *testing.T) {
	// 给一个不存在且不可创建的 signals_dir（用文件冒充目录）
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o600)
	// 让 signals_dir 是 blocker 下的子路径，mkdir 会失败
	badDir := filepath.Join(blocker, "subdir")

	var createdID uint64
	repo := &mockPackRepo{
		findFn: func(ctx context.Context, s string) (*entity.StickerPack, error) {
			return nil, apperr.ErrNotFound
		},
		createFn: func(ctx context.Context, pack *entity.StickerPack) error {
			pack.ID = 99
			createdID = pack.ID
			return nil
		},
	}
	uc := NewPackUseCase(repo, badDir)
	uc.telegramCheckURL = skipTelegramCheck(t)

	r, err := uc.AddManual(context.Background(), "BadSignal")
	if err == nil {
		t.Fatal("信号文件写入失败时应返错误")
	}
	if !stderrors.Is(err, apperr.ErrInternal) {
		t.Errorf("应是 ErrInternal, got %v", err)
	}
	// 但 pack 必须已经创建（不应回滚）
	if r == nil || r.Pack == nil || r.Pack.ID != 99 {
		t.Errorf("pack 必须已创建, got %+v", r)
	}
	if createdID == 0 {
		t.Error("Create 应被调用")
	}
}

func TestAddManual_RaceConflictReturnsExisting(t *testing.T) {
	// 模拟并发：FindByShortName 第一次 not_found，Create 撞 UNIQUE，再 FindByShortName 拿到
	existing := &entity.StickerPack{ID: 55, ShortName: "Race"}
	calls := 0
	repo := &mockPackRepo{
		findFn: func(ctx context.Context, s string) (*entity.StickerPack, error) {
			calls++
			if calls == 1 {
				return nil, apperr.ErrNotFound
			}
			return existing, nil
		},
		createFn: func(ctx context.Context, p *entity.StickerPack) error {
			return apperr.ErrConflict.WithMessage("duplicate")
		},
	}
	uc := NewPackUseCase(repo, t.TempDir())
	uc.telegramCheckURL = skipTelegramCheck(t)
	r, err := uc.AddManual(context.Background(), "Race")
	if err != nil {
		t.Fatalf("竞态应自愈, got %v", err)
	}
	if !r.AlreadyExisted || r.Pack.ID != 55 {
		t.Errorf("应返回已存在的 pack: %+v", r)
	}
}

// ---------------- FindByShortName ----------------

func TestFindByShortName_Reject(t *testing.T) {
	uc := NewPackUseCase(&mockPackRepo{}, "")
	_, err := uc.FindByShortName(context.Background(), "../bad")
	if !stderrors.Is(err, apperr.ErrInvalidParam) {
		t.Errorf("非法 short_name 应被拒, got %v", err)
	}
}

// ---------------- verifyPackExistsOnTelegram ----------------

func TestVerifyPackExists_NotExist(t *testing.T) {
	// 不存在的包：TG 返回 tg://addstickers 深链接
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `<a href="tg://addstickers?set=FakePack">Add Stickers</a>`)
	}))
	defer ts.Close()

	uc := &PackUseCase{}
	err := uc.verifyWithURL(context.Background(), ts.URL)
	if !stderrors.Is(err, apperr.ErrInvalidParam) {
		t.Fatalf("不存在的包应返回 ErrInvalidParam, got %v", err)
	}
	if err != nil && !stderrors.As(err, new(*apperr.AppError)) {
		t.Fatalf("应为 AppError, got %T", err)
	}
}

func TestVerifyPackExists_Exists(t *testing.T) {
	// 存在的包：TG 返回 tg://addemoji 深链接
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprint(w, `<a href="tg://addemoji?set=LoveIcons">Add Emoji</a>`)
	}))
	defer ts.Close()

	uc := &PackUseCase{}
	err := uc.verifyWithURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("存在的包应放行, got %v", err)
	}
}

func TestVerifyPackExists_NetworkError(t *testing.T) {
	// 连不上的地址，应放行不阻塞
	uc := &PackUseCase{}
	err := uc.verifyWithURL(context.Background(), "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("网络错误应放行, got %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

import axios from "axios";
import type { AxiosError, AxiosInstance } from "axios";
import { toast } from "sonner";
import type {
  AddPackRequest,
  AddPackResponse,
  ApiResponse,
  EmojiSticker,
  HealthStatus,
  ListPacksParams,
  PageResult,
  RandomEmojiParams,
  SearchParams,
  StatsOverview,
  StickerPack,
  StickerPackWithPreview,
} from "./types";

const baseURL = (import.meta.env.VITE_API_BASE as string | undefined) ?? "";

export const http: AxiosInstance = axios.create({
  baseURL,
  timeout: 15_000,
  headers: { Accept: "application/json" },
});

/** 后端返回结构里 code !== 0 也是 HTTP 200，需在 response 拦截器里抛 */
export class ApiError extends Error {
  code: number;
  status: number;
  raw: unknown;
  constructor(code: number, status: number, message: string, raw: unknown) {
    super(message);
    this.code = code;
    this.status = status;
    this.raw = raw;
  }
}

http.interceptors.response.use(
  (resp) => {
    const body = resp.data as ApiResponse<unknown> | undefined;
    if (body && typeof body === "object" && "code" in body) {
      if (body.code !== 0) {
        throw new ApiError(
          body.code,
          resp.status,
          body.message || "请求失败",
          body,
        );
      }
    }
    return resp;
  },
  (error: AxiosError<ApiResponse<unknown>>) => {
    if (error.response) {
      const body = error.response.data;
      const code = body?.code ?? error.response.status;
      const msg =
        body?.message ||
        (error.response.status >= 500
          ? "服务暂时不可用，请稍后重试"
          : error.message);
      return Promise.reject(
        new ApiError(code, error.response.status, msg, body),
      );
    }
    if (error.request) {
      return Promise.reject(
        new ApiError(-1, 0, "网络异常，请检查后端服务", error),
      );
    }
    return Promise.reject(new ApiError(-1, 0, error.message, error));
  },
);

/** 统一 toast 化 —— 业务可在 catch 之前选择拦截 */
export function toastApiError(err: unknown, fallback = "请求失败") {
  if (err instanceof ApiError) {
    toast.error(err.message || fallback);
    return;
  }
  if (err instanceof Error) {
    toast.error(err.message || fallback);
    return;
  }
  toast.error(fallback);
}

/* ---------------- 端点 ---------------- */

export async function searchEmoji(
  params: SearchParams,
  signal?: AbortSignal,
): Promise<PageResult<EmojiSticker>> {
  const resp = await http.get<ApiResponse<PageResult<EmojiSticker>>>(
    "/api/v1/search",
    { params, signal },
  );
  return resp.data.data;
}

export async function getEmojiById(
  id: string,
  signal?: AbortSignal,
): Promise<EmojiSticker> {
  const resp = await http.get<ApiResponse<EmojiSticker>>(
    `/api/v1/emoji/${encodeURIComponent(id)}`,
    { signal },
  );
  return resp.data.data;
}

export async function listPacks(
  params: ListPacksParams = {},
  signal?: AbortSignal,
): Promise<PageResult<StickerPack>> {
  const resp = await http.get<ApiResponse<PageResult<StickerPack>>>(
    "/api/v1/packs",
    { params, signal },
  );
  return resp.data.data;
}

// hot 排序 + preview_emojis 内联，给首页 HotPacks 用
export async function getHotPacks(
  limit = 12,
  signal?: AbortSignal,
): Promise<PageResult<StickerPackWithPreview>> {
  const resp = await http.get<ApiResponse<PageResult<StickerPackWithPreview>>>(
    "/api/v1/packs",
    {
      params: { sort: "hot", with_preview: true, page_size: limit },
      signal,
    },
  );
  return resp.data.data;
}

export async function getStats(signal?: AbortSignal): Promise<StatsOverview> {
  const resp = await http.get<ApiResponse<StatsOverview>>("/api/v1/stats", {
    signal,
  });
  return resp.data.data;
}

export async function getRandomEmojis(
  params: RandomEmojiParams = {},
  signal?: AbortSignal,
): Promise<PageResult<EmojiSticker>> {
  const resp = await http.get<ApiResponse<PageResult<EmojiSticker>>>(
    "/api/v1/emoji/random",
    { params, signal },
  );
  return resp.data.data;
}

export async function addPack(
  body: AddPackRequest,
): Promise<AddPackResponse> {
  const resp = await http.post<ApiResponse<AddPackResponse>>(
    "/api/v1/packs",
    body,
  );
  return resp.data.data;
}

export async function getHealth(): Promise<HealthStatus> {
  const resp = await http.get<HealthStatus>("/healthz");
  return resp.data;
}

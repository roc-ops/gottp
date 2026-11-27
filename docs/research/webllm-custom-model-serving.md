# WebLLM custom-model serving: download and memory budget

Research for [#40](https://github.com/roc-ops/gottp/issues/40) (parent map #37). Researched 2026-11-27 against
`@mlc-ai/web-llm@0.2.87`. "Source" below means the published npm tarball (`lib/index.js`, `lib/config.d.ts`,
`README.md`) of that exact version, which matches [mlc-ai/web-llm `src/`](https://github.com/mlc-ai/web-llm/tree/main/src)
(`config.ts`, `engine.ts`, `cache_util.ts`). Line numbers refer to `lib/index.js` in the tarball.

## TL;DR (decision-relevant)

1. **A fine-tuned Qwen2.5-Coder needs no custom compile.** If architecture and quantization stay the same (Qwen2, q4f16_1), the
   prebuilt WebGPU `model_lib` wasm can be reused. Only `mlc_llm convert_weight` + `gen_config` are needed, and the result
   is uploaded as a folder of shards. ([MLC docs][mlc-webllm])
2. **A self-hosted `model` URL must contain `/resolve/<something>/`.** Otherwise WebLLM appends `resolve/main/` to it
   (`cleanModelUrl`, index.js:11285). Either host the files at `…/<name>/resolve/main/…` or put `/resolve/main/` in the URL
   yourself. Relative URLs resolve against `document.URL`.
3. **Real download sizes (HF repo totals) at q4f16_1:** 0.5B **0.29 GB**, 1.5B **0.88 GB**, 3B **1.75 GB**, 7B **4.30 GB**.
   VRAM (`vram_required_MB`, prebuilt config): **945 / 1,630 / 2,505 / 5,107 MB**. The spike's "3B ≈ 2.5 GB download"
   matches the 3B *VRAM* figure, not the repo size, and should be re-checked.
4. **Hosting:** HF and any static host/CDN with `Access-Control-Allow-Origin` work. **GitHub Releases do not.** The
   `github.com/…/releases/download/…` hop returns no CORS header, so a browser `fetch` fails before the redirect.
   Range requests and HTTP cache headers are irrelevant, because WebLLM fetches each shard whole via `Cache.add()`, and the
   Cache API ignores HTTP caching headers.
5. **Storage is per origin (scheme+host+port) and best-effort by default.** Call `navigator.storage.persist()`, check
   `navigator.storage.estimate()` before downloading, and serve the editor from **one fixed origin**. Otherwise each new port
   or host re-downloads the model. WebGPU ships by default in Chrome/Edge (Win/macOS/ChromeOS; Linux only Intel Gen12+ and
   NVIDIA-Wayland), Safari 26 (macOS/iOS/iPadOS), and Firefox 141+ (Windows) / 145–147+ (Apple-silicon macOS). It is
   **not** on by default in Firefox Linux/Android.

---

## 1. How `appConfig.model_list` / `ModelRecord` points at weights and a model lib

`ModelRecord` in 0.2.87 (`lib/config.d.ts`; [src/config.ts](https://github.com/mlc-ai/web-llm/blob/main/src/config.ts)):

| field | meaning |
|---|---|
| `model` | Base URL of the weights folder. The comment shows HF forms only, but any URL works after `cleanModelUrl` (below). |
| `model_id` | Name passed to `CreateMLCEngine(model_id, {appConfig})`. It is also the cache key used by `hasModelInCache` and `deleteModelInCache`. |
| `model_lib` | URL of the compiled WebGPU `.wasm` model library. |
| `overrides` | Partial `mlc-chat-config.json` override, e.g. `context_window_size`. |
| `vram_required_MB` | Informational only (computed by `utils/vram_requirements`). The engine does not enforce it. |
| `low_resource_required`, `buffer_size_required_bytes` | Hints for limited devices and the needed `maxStorageBufferBindingSize`. |
| `required_features` | e.g. `["shader-f16"]`. Checked at load. A missing feature throws `ShaderF16SupportError` / `FeatureSupportError` (index.js:18292). |
| `integrity` | **New:** SRI hashes (`sha256-…`) for `config`, `model_lib` and tokenizer files, with `onFailure: "error"|"warn"` (`lib/integrity.d.ts`). Weight shards are not covered. |
| `model_manifest` | **New:** optional `mlc-model-manifest.json` URL, relative to `model`. It is used only when set. |

`AppConfig = { model_list, cacheBackend?: "cache"|"indexeddb"|"opfs"|"cross-origin", opfsAccessMode? }`.

What the load path does (`reload`, index.js:18220–18300):

- `cleanModelUrl(model)`: appends `/` and then, **if the URL does not match `/.+\/resolve\/.+\//`, appends `resolve/main/`**
  (index.js:11285–11291). A self-hosted `https://cdn.example/models/gottp-0.5b/` therefore becomes
  `…/gottp-0.5b/resolve/main/`. Either lay files out that way or write the URL as `…/gottp-0.5b/resolve/main/`.
- A non-`http` `model` is resolved against `document.URL`, which allows a same-origin relative path.
- Files fetched relative to the model URL are `mlc-chat-config.json`, `tensor-cache.json`, `params_shard_N.bin` and the tokenizer
  files. ([MLC docs][mlc-webllm])
- `model_lib` caching: a URL containing `localhost` is **not cached** (fetched fresh). A relative (non-`http`) URL is
  **not cached** by WebLLM and relies on normal HTTP caching. Any other absolute URL goes into the `webllm/wasm` cache. Note
  that `127.0.0.1` does not match the `localhost` check, so it *is* cached.

Minimal self-hosted record (fine-tuned 0.5B, reusing the prebuilt lib):

```js
import { CreateMLCEngine, modelLibURLPrefix, modelVersion } from "@mlc-ai/web-llm";
const appConfig = {
  model_list: [{
    model: "https://models.example.org/gottp-coder-0.5b-q4f16_1-MLC/resolve/main/",
    model_id: "gottp-coder-0.5b-q4f16_1-MLC",
    model_lib: modelLibURLPrefix + modelVersion + "/Qwen2-0.5B-Instruct-q4f16_1_cs1k-webgpu.wasm",
    vram_required_MB: 944.62,
    low_resource_required: true,
    overrides: { context_window_size: 4096 },
  }],
};
const engine = await CreateMLCEngine("gottp-coder-0.5b-q4f16_1-MLC", { appConfig });
```

### Reuse the prebuilt lib or compile one?

From the MLC docs ([Bring your own model to WebLLM][mlc-webllm]):

- A library is determined by **architecture, quantization, metadata (currently only `prefill-chunk-size`) and platform**.
  A fine-tune of a supported architecture (their example is a domain fine-tuned Llama 3) reuses the prebuilt lib, and only
  needs `mlc_llm convert_weight … --quantization q4f16_1` and `mlc_llm gen_config … --conv-template …`.
- Compiling your own (`mlc_llm compile … --device webgpu`) requires building `mlc_llm` from source plus emscripten.
- The prebuilt libs that 0.2.87's Qwen2.5-Coder records use (`modelVersion = "v0_2_87/base"`) are:
  - 0.5B → `Qwen2-0.5B-Instruct-q4f16_1_cs1k-webgpu.wasm`
  - 1.5B → `Qwen2-1.5B-Instruct-q4f16_1_cs1k-webgpu.wasm`
  - 3B → `Qwen2.5-3B-Instruct-q4f16_1_cs1k-webgpu.wasm`
  - 7B → `Qwen2-7B-Instruct-q4f16_1_cs1k-webgpu.wasm`

  They are hosted at `https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/` (0.5B wasm = 5.7 MB,
  `Access-Control-Allow-Origin: *`, `Cache-Control: max-age=300`, checked with `curl -I`).
- **Pin a version.** The doc comment says `prebuiltAppConfig` "is the only source of truth of which prebuilt model libraries
  are compatible with the current WebLLM npm version". A lib URL must match the npm version, so either pin
  `modelVersion` together with the npm version or vendor the wasm next to the weights.

## 2. Hosting options and HTTP requirements

How WebLLM fetches (Cache backend, index.js:6560–6575): `addToCache(url)` checks `cache.match(request)`, and on a miss
calls **`cache.add(new Request(url))`**. Per [MDN Cache.add()][mdn-cache-add] that is `fetch(url)` followed by `put`. It
rejects with `TypeError` on a non-2xx status, on an opaque (no-cors) response, or when the fetch fails. Consequences:

- **CORS is mandatory** for any host that is not the editor's own origin (`Access-Control-Allow-Origin` on every hop of
  every redirect).
- **Range requests are not used.** Each file is fetched whole, so `Accept-Ranges` does not matter. Keep shards modest; MLC
  shards are already ≤ ~270 MB (table below).
- **HTTP cache headers do not matter for WebLLM's copy**: "The caching API doesn't honor HTTP caching headers… they don't
  expire unless deleted" ([MDN Cache][mdn-cache]). Entries are keyed by full URL, so a new model version needs a new URL
  (e.g. HF commit hash in `resolve/<sha>/`) or an explicit `deleteModelAllInfoInCache(model_id)`.
- The spike's `Cache.add() network error` on a full disk is consistent with this path: the rejection surfaces as a
  fetch/put failure rather than a clearly labelled quota error. Spec'd quota failures are `QuotaExceededError`
  ([MDN quotas][mdn-quota]). Pre-check with `navigator.storage.estimate()`.

| Option | Works? | Evidence |
|---|---|---|
| **Hugging Face model repo** | Yes, and this is the path WebLLM is designed for. | `curl -I -H 'Origin: https://example.org'` on `…/resolve/main/mlc-chat-config.json` gives a 307 with `access-control-allow-origin: https://example.org`. On `params_shard_0.bin` it gives a 302 to `cdn.hf.co` (Xet bridge) with ACAO echoed, `accept-ranges: bytes`, `cache-control: no-store` (irrelevant, see above). Upload follows the [MLC docs][mlc-webllm] (`git clone` the HF repo, copy the converted folder, push). Repo layout already matches `resolve/<rev>/`. See also [HF model repos docs](https://huggingface.co/docs/hub/repositories). |
| **GitHub Releases** | **No** (browser fetch). | `curl -I -H 'Origin: …' https://github.com/cli/cli/releases/download/v2.40.0/…` gives a `302` to `release-assets.githubusercontent.com` with **no `Access-Control-Allow-Origin`**. A CORS preflight `OPTIONS` returns 404. The browser blocks the first hop. |
| **raw.githubusercontent.com** | Yes for small files such as the wasm. | `access-control-allow-origin: *`. This is what MLC uses for libs. It is unsuitable for GB of weights (git repo size limits / LFS). |
| **Own static host / CDN** (S3+CloudFront, R2, Pages, nginx) | Yes. | Needs ACAO for the editor origin (or `*`), 200 responses, and the `…/resolve/main/` path convention (§1). Same-origin hosting next to `editor/index.html` needs no CORS at all. |

## 3. Cache backends, per-origin behaviour, quota, eviction, persistence

WebLLM 0.2.87 offers four `cacheBackend` values ([README "Cache Backend Policy"][webllm-readme]; `lib/config.d.ts`):

- `"cache"`: the default, Cache API. The source notes this is "the most well-tested in WebLLM as of now".
- `"indexeddb"`: IndexedDB.
- `"opfs"`: Origin Private File System, with `opfsAccessMode` `async` (default), `sync` or `auto`. It fails hard when OPFS is unavailable.
- `"cross-origin"`: an experimental [WICG Cross-Origin Storage](https://github.com/WICG/cross-origin-storage) backend, which
  needs a Chrome extension. If the extension is missing it falls back to the Cache API (index.js:6985). This is the only
  backend that avoids per-origin re-downloads, and it is not something users can be expected to have.

Per-origin facts (all from [MDN, Storage quotas and eviction criteria][mdn-quota] unless noted):

- Storage is scoped **per origin = scheme + host + port**. `http://127.0.0.1:8791` and `http://127.0.0.1:8792` are
  different origins, which explains the spike's re-download. All four WebLLM backends are origin-scoped. **Implication:**
  pick one canonical origin for the editor and keep it fixed.
- Per-origin quota:

  | Browser | Best-effort | Persistent |
  |---|---|---|
  | Chrome/Edge | 60% of total disk (browser-wide cap 80%) | same |
  | Firefox | min(10% disk, 10 GiB) per eTLD+1 group | 50% of disk |
  | Safari 17+/macOS 14+ | ~60% of disk in the browser; ~15% in embedded WKWebViews | not differentiated |

  Quotas are computed from *total* disk, not free space. A full disk can therefore fail well under quota, which is the spike's
  failure mode. Exceeding quota throws `QuotaExceededError`.
- Chrome Incognito is limited to about 5% of disk, and about 300 MB if "clear site data on close" is set
  ([web.dev, Storage for the web][webdev-storage]). Private windows also discard data at session end.
- **Eviction:** under storage pressure or when the browser-wide limit is exceeded, browsers evict whole origins in LRU order,
  all data at once. Origins granted `navigator.storage.persist()` are skipped. **Safari also proactively deletes
  script-written storage (Cache API and IndexedDB included) for origins with no user interaction in the last 7 days of
  browser use**, when tracking prevention is on. This does not apply to installed home-screen web apps
  ([web.dev][webdev-storage]).
- **`persist()` grant:** Firefox shows a permission prompt. Chrome/Edge and Safari decide automatically from engagement,
  with no prompt.
- The Cache API needs a **secure context** (HTTPS, or `localhost`/`127.0.0.1`) ([MDN Cache.add()][mdn-cache-add]), and so
  does WebGPU ([MDN WebGPU][mdn-webgpu]). `file://` will not work.

Recommendation for gottp: keep `"cache"` (best tested). Call `persist()` once the user opts in to the model. Show
`estimate()` free space against the model size before the download. Catch `TypeError`/`QuotaExceededError` around load and
turn them into a "free disk space" message. Offer `deleteModelAllInfoInCache`.

## 4. Download size and VRAM: Qwen2.5-Coder at q4f16_1

Download is the sum of file sizes from `https://huggingface.co/api/models/mlc-ai/<repo>/tree/main`, measured 2026-11-27,
plus the wasm lib (~5–6 MB). VRAM is `vram_required_MB` from 0.2.87 `prebuiltAppConfig` (all with
`context_window_size: 4096`).

| Model (`mlc-ai/Qwen2.5-Coder-*-Instruct-q4f16_1-MLC`) | Download | Shards (largest) | `vram_required_MB` | `low_resource_required` |
|---|---|---|---|---|
| 0.5B | **0.29 GB** | 8 (68 MB) | **944.62** | true |
| 1.5B | **0.88 GB** | 30 (117 MB) | **1,629.75** | false |
| 3B | **1.75 GB** | 62 (156 MB) | **2,504.76** | true |
| 7B (reference) | 4.30 GB (4.0 GiB) | 88 (272 MB) | 5,106.67 | false |

q4f32_1 for comparison (VRAM only): 0.5B 1,060 MB, 1.5B 1,889 MB, 3B 2,894 MB, 7B 5,900 MB.

- The spike's **7B ≈ 4.1 GB** agrees with this. Its **3B ≈ 2.5 GB** does not: the repo totals 1.75 GB, while 2.5 GB
  equals the 3B VRAM number. This is probably a mix-up, or `estimate()` overhead; confirm before using it in UX copy.
- VRAM numbers are static estimates (weights + KV cache at 4k context + temp buffers). A longer `context_window_size` raises
  them, and they are not enforced. A device-lost error during `reload()` is the runtime symptom of running out
  (index.js:18300).

## 5. WebGPU availability and shader-f16

From the [gpuweb Implementation Status wiki][gpuweb-status] (edited 2026-10-02):

| Browser | Shipped by default | Not by default |
|---|---|---|
| Chrome / Edge | 113: Windows x64, macOS, ChromeOS. 121: Android 12+ (ARM/Qualcomm/Intel). 139: Android 16+ (Imagination). 144: Linux Intel Gen12+. 147: Linux NVIDIA (driver ≥ 535.183.01) on Wayland. | Other Linux setups and Windows ARM64 need flags (`--enable-unsafe-webgpu` …). Samsung Xclipse Android is TBD. |
| Safari | 26: macOS Tahoe, iOS, iPadOS, visionOS | Older OS versions |
| Firefox | 141: Windows. 145: Apple-silicon macOS 26+. 147: Apple-silicon on all macOS versions. | Intel Macs (Nightly only), Linux (Nightly only; planned for 2026), Android (behind pref) |

MDN still marks WebGPU **"Limited availability" (not Baseline)** and secure-context-only ([MDN WebGPU][mdn-webgpu]). It is
available in workers, so WebLLM's `WebWorkerMLCEngine` works.

**shader-f16:**

- WebLLM's device setup **always requests `shader-f16` when the adapter offers it** (index.js:~4840, "Always require f16 if
  available"). It only *fails* when a `ModelRecord` lists `required_features: ["shader-f16"]`.
- None of 0.2.87's prebuilt Qwen2.5-Coder q4f16_1 records list it (several Llama/Mistral/Hermes f16 records do). An f16 lib
  on a device without f16 would still fail at shader compile.
- Recommendation: copy whatever the matching prebuilt record does, and gate in the UI with
  `adapter.features.has("shader-f16")`. Fall back to the q4f32_1 variant (≈ +12–16% VRAM, larger download) when it is absent.
- `shader-f16` shipped in Chrome 120 ([Chrome blog][chrome-120]). Not all GPUs support it. MDN warns that features may be
  missing even where hardware supports them ([MDN GPUSupportedFeatures][mdn-features]).

## Sources

- WebLLM 0.2.87 npm package (`npm pack @mlc-ai/web-llm@0.2.87`) and repo: https://github.com/mlc-ai/web-llm (`src/config.ts`, `src/engine.ts`, `src/cache_util.ts`, `README.md`)
- mlc-webllm: https://llm.mlc.ai/docs/deploy/webllm.html
- webllm-readme: https://github.com/mlc-ai/web-llm#cache-backend-policy
- Prebuilt libs: https://github.com/mlc-ai/binary-mlc-llm-libs/tree/main/web-llm-models
- HF repo file listing API: https://huggingface.co/api/models/mlc-ai/Qwen2.5-Coder-3B-Instruct-q4f16_1-MLC/tree/main (and 0.5B/1.5B/7B)
- mdn-quota: https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria
- mdn-cache: https://developer.mozilla.org/en-US/docs/Web/API/Cache
- mdn-cache-add: https://developer.mozilla.org/en-US/docs/Web/API/Cache/add
- webdev-storage: https://web.dev/articles/storage-for-the-web
- mdn-webgpu: https://developer.mozilla.org/en-US/docs/Web/API/WebGPU_API
- mdn-features: https://developer.mozilla.org/en-US/docs/Web/API/GPUSupportedFeatures
- gpuweb-status: https://github.com/gpuweb/gpuweb/wiki/Implementation-Status
- chrome-120: https://developer.chrome.com/blog/new-in-webgpu-120

[mlc-webllm]: https://llm.mlc.ai/docs/deploy/webllm.html
[webllm-readme]: https://github.com/mlc-ai/web-llm#cache-backend-policy
[mdn-quota]: https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria
[mdn-cache]: https://developer.mozilla.org/en-US/docs/Web/API/Cache
[mdn-cache-add]: https://developer.mozilla.org/en-US/docs/Web/API/Cache/add
[webdev-storage]: https://web.dev/articles/storage-for-the-web
[mdn-webgpu]: https://developer.mozilla.org/en-US/docs/Web/API/WebGPU_API
[mdn-features]: https://developer.mozilla.org/en-US/docs/Web/API/GPUSupportedFeatures
[gpuweb-status]: https://github.com/gpuweb/gpuweb/wiki/Implementation-Status
[chrome-120]: https://developer.chrome.com/blog/new-in-webgpu-120

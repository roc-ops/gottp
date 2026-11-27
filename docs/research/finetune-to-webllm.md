# Fine-tune to browser: Kaggle QLoRA limits and the MLC/WebLLM conversion path

Research ticket: roc-ops/gottp#39 (parent map #37). All sources read **2026-11-27** unless noted.

Goal under study: fine-tune a small coder model (Qwen2.5-Coder 1.5B/3B, maybe Qwen3.5) on
~2k–10k examples (prompts up to ~6k tokens) to write TTP templates and small YANG schemas,
then run it in the browser with `@mlc-ai/web-llm` (0.2.87 pinned today).

## TL;DR (decision-relevant)

1. **No custom wasm needed.** A fine-tune of Qwen2.5-Coder that keeps the architecture and
   uses `q4f16_1` can reuse WebLLM's prebuilt model library. WebLLM's own catalogue already
   does this: `Qwen2.5-Coder-3B-Instruct-q4f16_1-MLC` points at `Qwen2.5-3B-Instruct-q4f16_1_cs1k-webgpu.wasm`,
   and Coder-1.5B points at `Qwen2-1.5B-Instruct-q4f16_1_cs1k-webgpu.wasm`. The pipeline is
   merge, then `convert_weight`, then `gen_config`, then host on HF. The `pip` nightly `mlc_llm` is enough;
   you don't need an emscripten/TVM source build.
2. **Kaggle is T4×2 only since 2026-09-15.** P100 was retired. The quota is about 30 GPU-h/week and sessions run up to 12 h.
   T4 has no bf16, so training runs in fp16. Users report poor T4 availability since the sunset.
3. **QLoRA on 1.5B/3B fits a single 16 GB T4 easily.** Unsloth puts 3B QLoRA at a ~3.5 GB minimum. The
   bottleneck is time. Rough estimate for 3B: ~1.5 h/epoch at 2k examples and ~7–8 h/epoch at 10k examples on one T4.
   Halve that for 1.5B, and DDP over both T4s can bring it down again. Measure before trusting this (see below).
4. **Context window is the hidden constraint.** Every WebLLM prebuilt Qwen entry overrides
   `context_window_size: 4096`, and no prebuilt in 0.2.87 goes above 4096. Our ~6k-token prompts
   need a runtime override to ~8192. MLC docs say only `prefill_chunk_size` is baked into the lib,
   so this *should* work with the cs1k lib, at about 0.3 GB extra KV cache for 3B. **This is unverified. Spike it first.**
5. **Qwen3.5 is a poor fit for the free-GPU path.** Unsloth says "It is not recommended to do QLoRA
   (4-bit) training on the Qwen3.5 models" and recommends bf16 LoRA, which a T4 cannot do natively.
   WebLLM does ship prebuilt Qwen3.5 0.8B/2B/4B/9B libs, so the browser side would work. The cheap-training side is the problem.

## 1. Kaggle limits

| Item | Value | Source / confidence |
|---|---|---|
| GPU types | **T4×2 only** (2× T4, 16 GB each). P100 retired **2026-09-15**. P100 notebooks are auto-switched to T4×2. Staff: no plan yet for other GPU options. | [Kaggle announcement "Sunsetting the NVIDIA Tesla P100 GPU on September 15, 2026"](https://www.kaggle.com/product-announcements/735239) (seen via search index; the page is JS-rendered) |
| Weekly GPU quota | "30 hours or sometimes higher depending on demand." T4×2 counts 1 h of quota per wall-clock hour, not per GPU. | [kaggle.com/docs/efficient-gpu-usage](https://www.kaggle.com/docs/efficient-gpu-usage), [T4 launch post](https://www.kaggle.com/product-feedback/361104) (via search index) |
| Session length | 12 h for CPU/GPU, 9 h for TPU | [kaggle.com/docs/notebooks](https://www.kaggle.com/docs/notebooks) (via search index) |
| T4×2 VM | 2× T4, 4 CPU cores, 29 GB RAM | [T4 launch post](https://www.kaggle.com/product-feedback/361104) (via search index) |
| Idle timeout | 60 min idle for interactive sessions. Background "Save & Run All" runs to the 12 h cap. | docs via search index |
| Disk / internet | `/kaggle/working` about 20 GB persisted output. Internet off by default and needs a phone-verified account to enable. | **Not verified.** The docs pages render client-side and I could not read the body. Treat as folklore until checked in the UI. |
| Competition-only GPUs | Some 2026 competitions (e.g. ARC Prize 2026) offer L4×4, which burns quota at 2× rate. Not available for general notebooks. | [ARC Prize 2026 page](https://www.kaggle.com/competitions/arc-prize-2026-arc-agi-2) (via search index) |
| Availability (recent) | User reports since the P100 sunset: accelerator selector greyed out, and "usable GPU time a fraction of the 30-hour quota." | [product-feedback thread](https://www.kaggle.com/discussions/product-feedback/743895). User report, not official. |

**Uncertain / changed recently:** the P100 removal is official and recent (Sep 2026). The
availability degradation is anecdotal. Don't plan a deadline around free Kaggle GPU time.

## 2. Does QLoRA on Qwen2.5-Coder 1.5B/3B fit a T4, and how long does it take?

**Fit: yes.** Unsloth lists 3B at **3.5 GB** for QLoRA and **8 GB** for 16-bit LoRA (both are
"absolute minimum") and supports compute capability ≥ 7.0, including T4
([Unsloth requirements](https://unsloth.ai/docs/get-started/beginner-start-here/unsloth-requirements)).
With a 6k-token sequence, batch size 1 and Unsloth gradient checkpointing, 3B should stay well under 16 GB.
Plain 16-bit LoRA (no 4-bit) on 1.5B/3B also fits a T4 and avoids the quantise-then-merge
rounding issue. **It's worth considering, because the models are small enough.**

**Precision:** T4 (Turing) has no bf16, so training is fp16 with loss scaling. Qwen2.5 usually
trains fine this way, but it is a known source of NaN or instability. Keep an eye on loss.

**Multi-GPU:** Unsloth supports DDP through `accelerate launch` / `torchrun`, but calls it manual
setup, with "official support coming soon"
([Unsloth multi-GPU](https://unsloth.ai/docs/basics/multi-gpu-training-with-unsloth)). Plain
TRL/PEFT + accelerate DDP also works on T4×2.

**Time per epoch: back-of-envelope estimate, not measured.**
Assumptions:
- Average example is about 2.5k tokens (≈2k prompt + 0.5k completion). 2k examples ≈ 5M tokens and
  10k examples ≈ 25M tokens per epoch. Prompt masking cuts loss but not compute.
- Training cost is about 6·N FLOPs/token, conservative for LoRA because base-weight grads are skipped but
  4-bit dequant adds overhead. Add ~15–20% for attention at ~6k length.
- T4 fp16 tensor peak is about 65 TFLOPS. Assume an effective ~15–20 TFLOPS (25–30% MFU) with Unsloth.

| Model | 2k examples (≈5M tok) | 10k examples (≈25M tok) |
|---|---|---|
| Qwen2.5-Coder-1.5B, 1× T4 | ~0.7–1 h | ~3.5–5 h |
| Qwen2.5-Coder-3B, 1× T4 | ~1.5–2 h | ~7–10 h |
| either, T4×2 DDP | ≈ ×0.55 of the above | ≈ ×0.55 |

What this means for Kaggle:
- A 3B run on 10k examples is about one epoch per 12 h session on a single T4. Checkpoint to `/kaggle/working` and resume.
- The weekly 30 h covers about 2–3 epochs of 3B at 10k, or many epochs of 1.5B at 2k.
- **Calibrate with a 50-step run** and extrapolate from measured tokens/s before committing quota.
  These numbers could be off by 2×.

## 3. LoRA fine-tune to WebLLM-loadable model: the exact path

Primary sources: [MLC docs, WebLLM: "Bring Your Own Model Variant"](https://github.com/mlc-ai/mlc-llm/blob/main/docs/deploy/webllm.rst),
[convert_weights.rst](https://github.com/mlc-ai/mlc-llm/blob/main/docs/compilation/convert_weights.rst),
[compile_models.rst](https://github.com/mlc-ai/mlc-llm/blob/main/docs/compilation/compile_models.rst),
[web-llm v0.2.87 `src/config.ts`](https://github.com/mlc-ai/web-llm/blob/v0.2.87/src/config.ts),
and the published `@mlc-ai/web-llm@0.2.87` npm tarball.

### Step 0: merge the adapter into a 16-bit HF checkpoint

MLC has no LoRA runtime in WebLLM, so you must ship merged weights. Merge into the **fp16/bf16
base**, not the 4-bit one. PEFT warns that on a quantized model "merging re-quantizes the base
weight" ([PEFT LoRA guide](https://huggingface.co/docs/peft/main/en/developer_guides/lora)).

```python
# plain PEFT: load base in fp16, attach adapter, merge
base = AutoModelForCausalLM.from_pretrained("Qwen/Qwen2.5-Coder-3B-Instruct", torch_dtype=torch.float16)
model = PeftModel.from_pretrained(base, "./lora-adapter").merge_and_unload()  # not in-place
model.save_pretrained("./merged"); tokenizer.save_pretrained("./merged")
# or Unsloth:
model.save_pretrained_merged("merged", tokenizer, save_method="merged_16bit")
```
(Unsloth call quoted from the [Unsloth Qwen3.5 fine-tune guide](https://unsloth.ai/docs/models/qwen3.5/fine-tune).)

### Step 1: install mlc_llm (prebuilt wheel, no source build)

```bash
python -m pip install --pre -U -f https://mlc.ai/wheels mlc-llm-nightly-cpu mlc-ai-nightly-cpu
# CUDA wheels also exist: -cu128 / -cu130. macOS: the cpu wheel includes Metal.
```
([MLC install docs](https://github.com/mlc-ai/mlc-llm/blob/main/docs/install/mlc_llm.rst)).
A source build plus emscripten is needed **only** for `mlc_llm compile --device webgpu`.
The docs say: "To compile model libraries for webgpu, you need to build mlc_llm from source".
The variant path below never compiles.

### Step 2: convert_weight to q4f16_1

```bash
mlc_llm convert_weight ./merged --quantization q4f16_1 -o dist/gottp-coder-3b-q4f16_1-MLC
```

### Step 3: gen_config with the Qwen conversation template

```bash
mlc_llm gen_config ./merged --quantization q4f16_1 --conv-template qwen2 \
    --context-window-size 8192 -o dist/gottp-coder-3b-q4f16_1-MLC
```
- `qwen2` is the template name in the prebuilt `mlc-ai/Qwen2.5-Coder-3B-Instruct-q4f16_1-MLC`
  `mlc-chat-config.json` (`"conv_template": {"name": "qwen2", ...}`, ChatML `<|im_start|>`).
  Train with the same ChatML format. If training used a custom system prompt, set it in
  `mlc-chat-config.json` or pass it per request.
- **Recommended check:** diff the generated `mlc-chat-config.json` against the prebuilt one
  (<https://huggingface.co/mlc-ai/Qwen2.5-Coder-3B-Instruct-q4f16_1-MLC/raw/main/mlc-chat-config.json>).
  `model_type: qwen2`, shapes and quantization must match for the prebuilt lib to load.

### Step 4: files to host

`dist/<name>-MLC/` contains `mlc-chat-config.json`, `tensor-cache.json`, `params_shard_*.bin`,
`tokenizer.json`, `tokenizer_config.json`, `vocab.json` and `merges.txt`. Push the directory to a HF
repo, or any static host with CORS. Sizes for the prebuilt equivalents: **3B ≈ 1.75 GB, 1.5B ≈ 0.88 GB**
(HF API, summed file sizes).

> **Changed recently:** the weight index was renamed from `ndarray-cache.json` to
> **`tensor-cache.json`**. The web-llm 0.2.87 bundle hard-codes `TENSOR_CACHE_MANIFEST_FILENAME = "tensor-cache.json"`
> (checked in the npm tarball). Old mlc-ai HF repos carry both files. Use a current mlc_llm
> nightly so the converter writes `tensor-cache.json`.
> 0.2.87 also adds an **opt-in** `mlc-model-manifest.json` artifact manifest
> ([model_artifact_manifest.rst](https://github.com/mlc-ai/mlc-llm/blob/main/docs/compilation/model_artifact_manifest.rst)):
> "A model without one loads from `mlc-chat-config.json` as before." It's not needed.

### Step 5: register in WebLLM, reusing the prebuilt Qwen lib

```ts
import { CreateMLCEngine, modelLibURLPrefix, modelVersion } from "@mlc-ai/web-llm";
const appConfig = { model_list: [{
  model: "https://huggingface.co/<org>/gottp-coder-3b-q4f16_1-MLC",
  model_id: "gottp-coder-3b-q4f16_1-MLC",
  model_lib: modelLibURLPrefix + modelVersion + "/Qwen2.5-3B-Instruct-q4f16_1_cs1k-webgpu.wasm",
  // 1.5B: "/Qwen2-1.5B-Instruct-q4f16_1_cs1k-webgpu.wasm"
  overrides: { context_window_size: 8192 },   // prebuilt uses 4096; see the risk below
}]};
const engine = await CreateMLCEngine("gottp-coder-3b-q4f16_1-MLC", { appConfig });
```
In 0.2.87, `modelVersion = "v0_2_87/base"` and the prefix is
`https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/`. I confirmed the
3B wasm URL returns HTTP 200. The wasm is tied to the WebLLM runtime version, so **pin the
lib URL to the WebLLM version you ship**, or self-host the wasm. Bumping web-llm (0.2.88 and 0.2.89
are out, released 2026-11-10 and 2026-11-17) can change `modelVersion`.

### When a custom wasm *is* required

Per the MLC docs, a lib is specified by architecture, quantization, `prefill_chunk_size` and platform
("currently only `prefill-chunk-size` affects the compiled model"). You must compile your own lib only if you:
- change the architecture or shapes,
- use a quantization with no prebuilt lib, or
- want a prefill chunk other than 1k.

Compiling needs `mlc_llm compile <cfg> --device webgpu -o x.wasm` from a source build with emscripten.

### Risk: 6k-token prompts vs. the 4096 default

- Every WebLLM 0.2.87 prebuilt sets `context_window_size` ≤ 4096. Counts across `config.ts`:
  127× 4096, 10× 2048, 26× 1024, none higher.
- `ChatOptions extends Partial<ChatConfig>`, so `context_window_size` can be overridden. Because the
  context window isn't compiled into the lib, an 8192 override should work.
- KV-cache cost for 3B (36 layers, 2 KV heads, head_dim 128, fp16) is about 36 KB/token, so roughly 0.3 GB at 8k.
- **No source confirms an 8k run with the cs1k Qwen lib.** Prove it in a 30-minute browser spike before
  training. Otherwise trim prompts to under 4k.

## 4. Alternatives to Kaggle

| Option | Cost | Limits / notes | Source |
|---|---|---|---|
| **Colab free** | $0 | Up to 12 h runtime. GPU access "heavily restricted". Limits are dynamic and unpublished. GPU type not guaranteed (typically T4). | [Colab FAQ](https://research.google.com/colaboratory/faq.html) |
| **Colab Pro / Pro+ / PAYG** | compute-unit based | Pro+ runs up to 24 h continuous with enough units. Premium GPUs subject to availability. **Exact prices not verified**: the signup page renders client-side. Historically ~$9.99/mo for 100 CU (Pro) and ~$49.99/mo for 500 CU (Pro+). Check before budgeting. | [Colab FAQ](https://research.google.com/colaboratory/faq.html), [signup](https://colab.research.google.com/signup) |
| **Modal** | Per-second billing. T4 $0.000164/s (≈$0.59/h), L4 ≈$0.80/h, A10 ≈$1.10/h, L40S ≈$1.95/h, A100-40G ≈$2.10/h, A100-80G ≈$2.50/h, H100 ≈$3.95/h. **Starter plan: $30/month free credits.** | Scriptable and reproducible. A 3B × 10k-example epoch on an L40S/A100 (bf16, ~4–6× a T4) probably costs about $3–8, which fits inside the free credits. | [modal.com/pricing](https://modal.com/pricing) |
| **RunPod** | Per-second billing. Community / Secure per hour: RTX 4090 $0.34/$0.89, RTX 3090 $0.22/$0.50, L4 $0.44/$0.69, A40 $0.35/$0.59, L40S $0.79/$1.09, A100 80G PCIe $1.19/$2.09, H100 PCIe $1.99/$2.89 | The page says it was updated 2026-11-26. Community prices are "from" prices. A 4090 runs bf16 and is several times faster than a T4 for about $0.34/h. | [runpod.io/pricing](https://www.runpod.io/pricing) |
| **Local Apple Silicon (MLX-LM)** | $0 marginal | `mlx_lm.lora --model <m> --train --data <dir>` supports `lora`, `dora` and `full`. You get QLoRA automatically if the model is quantized. Takes chat or completions JSONL, and `--mask-prompt` trains on completions only. `mlx_lm.fuse` merges adapters. Memory levers: `--batch-size`, `--num-layers` (default 16), `--grad-checkpoint`. The docs' reference point is about 250 tok/s on an M1 Max at batch 1 with 4 layers, which is slow: 25M tokens takes about 28 h. **Use the fused output as fp16 HF weights for `mlc_llm convert_weight`.** Fusing from a quantized base needs dequantising. Check `mlx_lm.fuse --help` for `--dequantize`; the LORA.md page doesn't document it. | [mlx-lm LORA.md](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/LORA.md) |

## 5. Qwen3.5 note

- WebLLM 0.2.87 ships prebuilt `Qwen3.5-{0.8B,2B,4B,9B}-q4f16_1-MLC` libs (`config.ts`), so the same
  "variant reuse" path applies on the browser side.
- Training side: Unsloth says "It is not recommended to do QLoRA (4-bit) training on the Qwen3.5
  models, no matter MoE or dense". Its bf16 LoRA VRAM figures are 0.8B: 3 GB, 2B: 5 GB and 4B: 10 GB
  ([Unsloth Qwen3.5 fine-tune](https://unsloth.ai/docs/models/qwen3.5/fine-tune)).
- T4 lacks bf16, so Qwen3.5 means 16-bit LoRA in fp16 on Kaggle, which is unvalidated, or paying for a bf16 GPU
  (Modal L4/L40S, or a RunPod 4090).
- mlc-llm main is still actively landing Qwen3.5 fixes (e.g. "Support float32 projection accumulation in
  Qwen3.5", commit 2026-11-21), and WebLLM 0.2.87 notes "Fix JSON schema generation for Qwen3.5".
  It's a moving target.

## 6. Open questions / what to spike next

1. Load a converted (even un-fine-tuned) Qwen2.5-Coder-3B with `context_window_size: 8192` against
   the cs1k lib in Chrome, and feed it a 6k-token prompt. This is the go/no-go for prompt length.
2. Run 50 training steps on Kaggle T4 (and on a Modal L4) to replace the time estimates with measured tokens/s.
3. Confirm `/kaggle/working` size and the internet/phone-verification rule in the live Kaggle UI.
4. Confirm current Colab prices on the signup page if Colab is shortlisted.

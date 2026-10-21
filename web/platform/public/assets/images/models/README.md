# Model icons

The explicit registry in `src/features/models/model-icon-registry.ts` assigns artwork to all 103 IDs in the current generated workspace catalog, including pending models. It does not change availability, names, prices, capabilities or provider routes. Unknown future IDs still use supplied artwork or the existing theme-aware fallback; the coverage test flags additions to the catalog fixture.

Versions of the same family share a mark. Where no dedicated family asset was selected, the developer mark is used. These are service/family identifiers, not separate official logos for every model version.

| Models | Mark | Reference |
| --- | --- | --- |
| Branded site chat (`chatgpt`) | Configured site logo, `brandLogoPath` in `src/config/brand.ts` | Internal brand asset; the legacy ID is not the ChatGPT product |
| GPT, GPT Image, GPT-4o Mini TTS, Whisper | OpenAI | [Brand](https://openai.com/brand/) |
| Seedream, Seedance | ByteDance developer mark | [Seed](https://seed.bytedance.com/en/) |
| Midjourney | Midjourney | [Service](https://www.midjourney.com/) |
| FLUX image/video | FLUX family | [Black Forest Labs models](https://bfl.ai/models) |
| Nano Banana, Imagen, Veo, Lyria | Google developer mark | [Google DeepMind models](https://deepmind.google/models/) |
| Gemini text and Omni video | Gemini family | [Google DeepMind models](https://deepmind.google/models/) |
| Claude Opus, Sonnet, Haiku, Fable | Claude family | [Anthropic](https://claude.com/product/overview) |
| Qwen text/image | Qwen family | [Qwen](https://qwen.ai/) |
| Grok text/image/video | Grok family | [Grok](https://x.ai/grok) |
| DeepSeek | DeepSeek | [Service](https://www.deepseek.com/) |
| Kimi | Kimi | [Service](https://www.kimi.com/) |
| Kling | Kling | [Service](https://klingai.com/) |
| MiniMax H3 | MiniMax developer mark | [MiniMax](https://www.minimax.io/) |
| Runway Gen-4.5 | Runway | [Service](https://runway.com/) |
| HappyHorse | HappyHorse family | [Alibaba Cloud](https://www.alibabacloud.com/help/en/model-studio/happyhorse-1-0-i2v) |
| SkyReels | Skywork developer mark | [Kunlun](https://www.kunlun.com/en/index.html) |
| Suno | Suno | [Service](https://suno.com/) |
| Wan, Z-Image | Alibaba Cloud developer mark | [Wan](https://home.alibabagroup.com/en-US/ai-governance/wan), [Tongyi Z-Image](https://github.com/Tongyi-MAI/Z-Image) |
| PixVerse | PixVerse | [Service](https://pixverse.ai/) |
| Vidu | Vidu | [Service](https://www.vidu.com/) |

## Source and presentation

The 20 SVGs come from the maintained [Lobe Icons collection](https://github.com/lobehub/lobe-icons), pinned to commit `8a1b2821bb768c484fe8da20c5e1838b6bf551a9`, downloaded on 2026-10-18. These are vector renditions from the collection, not vendor-supplied model-version logo packs. No package or external runtime resource is needed.

The local presentation adds a white rounded canvas and 4 units of clear space (`viewBox="-4 -4 32 32"`); original mark paths, gradients and aspect ratios are unchanged. Monochrome marks use a fixed dark fill; original colored marks retain their colors. No raster upscaling, scripts, external images, links or font dependencies. Shared families reuse the same local URL, so the browser can reuse cached files across cards and selectors. The existing site chip is reused as a PNG; it is not represented as a vector.

Marks identify the respective services; no endorsement or partnership is implied. The MIT license below covers the icon implementation; trademarks remain with their owners.

## Pinned files and SHA-256

Source paths are relative to `packages/static-svg/icons/` in the pinned [upstream tree](https://github.com/lobehub/lobe-icons/tree/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons).

| Local file | Upstream file | Original SHA-256 | Local SHA-256 |
| --- | --- | --- | --- |
| openai.svg | [openai.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/openai.svg) | `a595df6b423920c67a7f8f73c063e4bfb72d415948097b6cac063a2366bb5186` | `6609bd7dd385dbb41ec2159aaee5be95e02a4ca48671997778f42596826a6b6d` |
| bytedance.svg | [bytedance-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/bytedance-color.svg) | `3d37647be262c79b044e293bb1a4993202537e9f9ffdbd2bbe7e9fc5378d1005` | `a3c6b041bd3c6ea4d46b386a6837eec66d5791c881c1a67a5148c1cb31415780` |
| midjourney.svg | [midjourney.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/midjourney.svg) | `e09640ccbf6f643b4624e540f198cd704a6bf3d75dc43b0938628caf4e98c30b` | `4cd5f097e3e1cd8f2f4a4ea786d96dfef9fff490ab943eb8c8ff74501e148f26` |
| flux.svg | [flux.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/flux.svg) | `c7d557466f39a895b1cd0d233a64c092ef61ba052160af7db20af37464a9138a` | `27e32d9a6cb533c027c09de38308aa1ca9fc96f28421b6b392f1ddeb5bae611d` |
| google.svg | [google-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/google-color.svg) | `4e6aa8892ef15a6b431f40f2c9045979bf7e2357524e7bbb89b353cf4f32a5ac` | `3da43eae7d179753e72949989b3f3eb22628a864d12173d6bb24f1d924bdf696` |
| qwen.svg | [qwen-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/qwen-color.svg) | `77f5768c66d08ce1d3d14e73373975c1bc0454be88c81523ddd0ffd7e2974029` | `51bcf23adb9f5ee19fb2522661d6df915a378fcc5ad9c92723c745c277a76650` |
| grok.svg | [grok.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/grok.svg) | `9175fc90c22655160231976c849f25a03b888d7cc0e04c5f1b987b659bb07c95` | `2908f39219da88a85bbaf61063f5b1583595ea5746905c2f767e6b4cc0f9be93` |
| claude.svg | [claude-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/claude-color.svg) | `a3101f3047a119aa11825ad9369510f0c472428c8c52d420e31bc62db44a8364` | `7ad0d618fa85a27fa5bb70c76b3fe1ce4c2d5691ec6875a9d53e7ee35e719372` |
| gemini.svg | [gemini-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/gemini-color.svg) | `8ab0a9bafec11f7e69bcb9fc4ffd8f1bc927d1ddcbbb6ff36dee5ae8b5a9d602` | `2eaa25a0e959a2ba47c70c73aab05932465447d435e2710c3e75f2335ed119a4` |
| kling.svg | [kling-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/kling-color.svg) | `a8aa4def0eb1c3ca0f0528597d28fa495249aa710aa403d3c571c47c81c6150c` | `c3a87c2ca6486208efc423bdf23e0b8bc39f372ec42f8868120f14e3c60082de` |
| minimax.svg | [minimax-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/minimax-color.svg) | `7f7187fa6d9b341ac5f22f6c5a970523afb439dc45566d64eb4f4b9b604d6b01` | `61054556fce0ee91e33445dfe637ee620b02cd477dbdf0447317fcd147475aa2` |
| runway.svg | [runway.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/runway.svg) | `dba3459264af7b4fffb5c4bfc909748e1ec1f122ffcaf0a8fbb626781888eaf2` | `acf63e16b43a38ddaf954f65e6fac97d548691ceeabdc9cdd6e020ca21f136e8` |
| happyhorse.svg | [happyhorse.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/happyhorse.svg) | `11a19089dca18a99a69666fd719f6233b58d36d6d95d7f96277d5f8354b95c20` | `1a96811fca81cccf91862443f32293fcf4c3219b0d20001fec4612965eb36ee8` |
| skywork.svg | [skywork-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/skywork-color.svg) | `fd389cf920bc9e81866d468f008e6571a02ca8c0dbf3db799041dc7905913ac8` | `089094787e57dce16b6cddd1ed0c4f98ef160c0a679da0d99451a689b560a9c4` |
| suno.svg | [suno.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/suno.svg) | `591debb0ed912ff6cb89fc66a05f58e3291eb383c691c9dbf717927559d4eafc` | `7d626accea832fe27e7b0491259a8894e1b957f88ce6c30cee82f35320eca295` |
| alibaba.svg | [alibabacloud-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/alibabacloud-color.svg) | `73ea97baf919edb3d888a77cd45b21574e190a80124a59e199ea6953a28a959c` | `f7c886e2bf033d027a3a90d98f57826f01f95296e41ce9b280f1cab87c543d26` |
| pixverse.svg | [pixverse-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/pixverse-color.svg) | `48f2fdfa9b77845257cde932ea4c04895088e145bb61d94b309d5ef3b2751bdc` | `ca16fd1d1061de2544de7623c0877b748d37118a10147e28d4991763520e9260` |
| vidu.svg | [vidu-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/vidu-color.svg) | `66024ae1b72a813287a82bd02ee218258265386ef350c3995ca7bd966e36ec56` | `de5772db49597bb7f904f8636bf8e63497641635089f4c57f71b7243f2e2929a` |
| deepseek.svg | [deepseek-color.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/deepseek-color.svg) | `deba5f98a5c1796e20fcac3149bcd7eb8a32f0bdd04d048819400b1f28bd1439` | `f4cac083a9f7259c2b5268bb5766ac294266dd808502e5335064168d52a3872c` |
| kimi.svg | [kimi.svg](https://github.com/lobehub/lobe-icons/blob/8a1b2821bb768c484fe8da20c5e1838b6bf551a9/packages/static-svg/icons/kimi.svg) | `ed08131b46acf70e6e2144eceb775e74098b58ea65fa21eaee04be7b05cfe63b` | `7503e78739cf192b91272ed1e42f4b8be17a59be7a4c6b515ac1bddd90d79128` |

## Upstream license

```text
MIT License

Copyright (c) 2023 LobeHub

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

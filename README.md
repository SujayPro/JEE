<p align="center">
  <img src="https://jee.money/assets/logo/jee-chain.png" width="88" alt="JEE Money" />
</p>

<h1 align="center">JEE Money</h1>

<p align="center">
  <strong>Zero-fee Layer-1 · powered by JEE</strong>
</p>

<p align="center">
  <a href="https://jee.money"><img src="https://img.shields.io/badge/website-jee.money-D4AF37?style=for-the-badge" alt="Website" /></a>
  <a href="https://jeescan.org"><img src="https://img.shields.io/badge/explorer-jeescan.org-D4AF37?style=for-the-badge" alt="Explorer" /></a>
  <img src="https://img.shields.io/badge/chain%20ID-JEE-D4AF37?style=flat-square" alt="Chain ID" />
  <img src="https://img.shields.io/badge/Cosmos%20SDK-0.50-6E44FF?style=flat-square" alt="Cosmos SDK" />
  <img src="https://img.shields.io/badge/CometBFT-0.38-00ADD8?style=flat-square" alt="CometBFT" />
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square" alt="Go" />
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue?style=flat-square" alt="License" />
</p>

<p align="center">
  <a href="https://jee.money">jee.money</a> ·
  <a href="https://jeescan.org">jeescan.org</a> ·
  <a href="chain-registry/jeechain/chain.json">Chain metadata</a> ·
  <a href="config/genesis-wallets.json">Genesis allocations</a>
</p>

---

## Overview

**JEE CHAIN** is a Cosmos SDK application with **zero transaction fees** for users. Instead of paying gas in JEE, accounts spend **Mana** — a regenerating per-block budget — so everyday transfers stay free while the network stays bounded and fair.

| | |
|---|---|
| **Chain ID** | `JEE` |
| **Node binary** | `jeed` |
| **Bech32 prefix** | `jee` |
| **Native token** | **JEE** (display) · on-chain denom `jeff` · 6 decimals |
| **Block target** | ~1 second |
| **Fee model** | `0` min gas price · Mana-gated throughput |

Public endpoints, genesis, and wallet setup live on **[jee.money](https://jee.money)** — not in this repo.

---

## How Mana works

JEE CHAIN does **not** refill a fixed “daily allowance” at midnight. Mana is a **bandwidth meter** that **regenerates every block** (~1 second). Each transaction **spends** mana instead of paying JEE fees. Your meter refills until it hits a **cap** that grows with your JEE balance.

| Concept | Behavior |
|---------|----------|
| **Spend** | Every tx costs mana: `100 + tx_bytes` (no coin fees) |
| **Regen** | Mana added **per block**, not once per day |
| **Cap** | Max mana you can hold — higher if you hold more JEE |
| **Floor** | Every wallet gets at least **1,000,000** mana cap |

### Spam protection (per wallet + network safety)

Limits are **per account**, so a flooder mainly hurts **themselves**. Other users are not charged extra JEE.

| Layer | What it does |
|-------|----------------|
| **Mana** | Flooder runs out of bandwidth; txs fail with *insufficient bandwidth* |
| **20 txs / block / account** | Hard cap per wallet per block (bot or manual spam) |
| **Tx size cost** | Larger txs cost more mana (`100 + bytes`) |
| **Adaptive PoW** | Only when the **whole chain** is very busy (>1,000 txs in a block); calm network = no PoW, normal wallets unaffected |

Under heavy **network-wide** load, proof-of-work can engage temporarily until traffic calms — then it turns off again.

### Parameters (from chain code & genesis)

| Parameter | Value |
|-----------|--------|
| Minimum cap (any wallet) | **1,000,000** mana |
| Minimum regen | **1,000 mana per block** |
| Blocks per day (~1s blocks) | **~86,400** |
| Max txs per wallet per block | **20** |
| Typical simple tx cost | **~400–800** mana |
| Total mana pool (network) | **1,000,000,000** |

### Small / new wallet (little or no JEE)

- **Cap:** 1,000,000 mana
- **Refill:** 1,000 mana/block → full tank from empty in **~1,000 blocks (~17 minutes)**
- **Daily throughput:** you only *hold* 1M at once, but you can cycle refills — on the order of **~80–90 million mana spent per day** if you keep transacting
- **Simple sends:** **~100k+** small txs/day if mana were the only limit; **20 txs/block** hard-caps at **~1.7 million txs/day** per wallet

### More JEE = higher cap

```text
mana cap ≈ (your JEE balance / total supply) × 1,000,000,000
```

Minimum cap is always **1,000,000** even with zero balance.

Examples at **247M JEE** total supply:

| Wallet size | Approx mana cap |
|-------------|-----------------|
| ~0 JEE (floor) | 1,000,000 |
| ~30.9M JEE | ~125,000,000 |
| ~96M JEE | ~388,000,000 |

Regen is **at least 1,000 mana/block** for most wallets; very large holders can regen faster than the floor.

### Genesis bonus

Selected treasury accounts (e.g. founder, validator) start with **500,000,000** mana in genesis — extra headroom at launch only.

**In one line:** zero JEE fees, mana is the limit, spam punishes the spammer first, and a typical small wallet holds **~1M mana** refilling at **~1,000 mana per second** — not a fixed daily lump sum.

Implementation: [`x/mana/`](x/mana/) · ante pipeline: [`app/ante/ante.go`](app/ante/ante.go)

---

## Token supply

**Fixed genesis supply: 247,000,000 JEE**

| Allocation | JEE | Share | Purpose |
|------------|-----|-------|---------|
| Community | 96,057,830 | 38.9% | Grants, airdrops, ecosystem |
| Public reserve | 81,708,070 | 33.1% | Partners, liquidity, future sale |
| Founder | 30,875,000 | 12.5% | Founder allocation |
| Foundation | 25,836,200 | 10.5% | Governance & development |
| Validator | 12,522,900 | 5.1% | 10M staked · ~2.5M liquid ops |

Treasury addresses: [`config/genesis-wallets.json`](config/genesis-wallets.json)

---

## Validator

| | |
|---|---|
| **Launch** | Single founding validator at genesis |
| **Self-delegation** | 10,000,000 JEE |
| **Commission** | 10% (max 20%) |
| **Moniker** | `jee-mainnet` |

Additional validators can join via standard Cosmos staking once the network is public.

---

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Wallet    │────▶│  jeed (app)  │────▶│  CometBFT BFT   │
│  Keplr etc. │     │  Cosmos SDK  │     │  ~1s blocks     │
└─────────────┘     └──────┬───────┘     └─────────────────┘
                           │
                    ┌──────▼───────┐
                    │  x/mana      │  zero-fee lane
                    │  regen / cap │  per account per block
                    └──────────────┘
```

**Modules:** `auth`, `bank`, `staking`, `gov`, `mint`, `slashing`, `evidence`, `upgrade`, **`mana`** (custom).

---

## Build from source

**Requirements:** Go 1.26+ ([download](https://go.dev/dl/)), optional [Buf](https://buf.build) for protos.

```bash
git clone https://github.com/SujayPro/jee-chain.git
cd jee-chain

# Local dev binary
make build
# → build/jeed

# Linux ARM64 (typical cloud validator)
make build-linux-arm64
# → build/jeed-linux-arm64  (~70 MB stripped)
```

```bash
# Local single-node devnet
make init
make start
```

---

## Wallet

Use the hosted helper (pulls live RPC/REST from jee.money):

**[jee.money → Add to Keplr](https://jee.money/assets/add-keplr.html)**

Or import manually from [chain registry metadata](chain-registry/jeechain/chain.json) and [genesis](https://jee.money/genesis.json).

---

## Repository layout

| Path | Open source? | Notes |
|------|----------------|-------|
| `app/`, `cmd/`, `x/`, `proto/` | **Yes** | Chain implementation |
| `go.mod`, `Makefile`, `buf.yaml` | **Yes** | Build & tooling |
| `config/genesis.json` | **Yes** | Public genesis state |
| `config/genesis-wallets.json` | **Yes** | Allocation reference |
| `config/app.toml`, `config/config.toml` | **Templates** | Sanitized — set `external_address` on your node |
| `assets/logo/`, `add-keplr.html` | **Yes** | Branding & wallet page |
| `chain-registry/` | **Yes** | Cosmos chain-registry format |
| `build/` | **No** | Compiled binaries — build locally |
| `deploy/` | **Operator-only** | Server scripts & nginx — not needed to build chain |
| `config/priv_validator_key.json` | **Never** | Validator key — gitignored |
| `config/node_key.json` | **Never** | P2P node key — gitignored |
| `.jeechain/`, `.regen-gentx/` | **Never** | Local node data / scratch |

**Owner minimum to run mainnet:** `build/jeed-linux-arm64` + `config/` (genesis, tomls, keys on server only) + `deploy/` scripts on the machine — **not** the Go source tree.

**Contributors / auditors need:** everything in the first rows — no `deploy/`, no keys, no `build/`.

---

## What you can skip

- **`deploy/nginx/`** — reverse-proxy configs for your host; irrelevant to chain logic  
- **`deploy/*.sh`** — one-off operator automation; safe to omit from forks  
- **`x/mana/*_test.go`** — tests only; not required to run a node  
- **`.cursor/`** — editor rules; not part of the chain  
- **Duplicate uploads** — never ship `jeed.exe`, test logs, or `.har` captures  

---

## Links

| Resource | URL |
|----------|-----|
| Website | [jee.money](https://jee.money) |
| Explorer | [jeescan.org](https://jeescan.org) |
| Genesis | [jee.money/genesis.json](https://jee.money/genesis.json) |
| Chain registry | [`chain-registry/jeechain/`](chain-registry/jeechain/) |

Node operators: network seeds and RPC endpoints are published on **jee.money**, not hardcoded here.

---

## License

[Apache License 2.0](LICENSE)

---

<p align="center">
  <sub>JEE Money · zero-fee blockchain · JEE CHAIN</sub>
</p>

# Contributing to shikA

Thanks for wanting to help build this. It's early, which means high-leverage contributions are wide open.

## Getting started

Requirements: [Go 1.22+](https://go.dev/dl/). Optionally [prima.cpp](https://github.com/OpenCPIL/prima.cpp) built locally to test real inference.

```bash
git clone https://github.com/braymix/shika.git
cd shikA
make run          # runs the orchestrator; dashboard at http://localhost:8977
make build        # produces ./bin/shikad
make test         # go test ./...
make check        # gofmt + go vet
```

To simulate a mesh on one machine, run several instances with different API ports and names:

```bash
go run ./cmd/shikad -name node-a &
# in another shell, edit a config to set api_addr to 0.0.0.0:8978 and run node-b
```

(Multicast loopback behaviour varies by OS; for reliable single-host testing use `seeds` pointing each instance at the others' control addresses.)

## Where help is most useful

See [ROADMAP.md](ROADMAP.md). Especially wanted right now:

- **Phase 1**: real end-to-end launch of prima.cpp across two devices, and start-ordering via the control API.
- **Platform testing**: does discovery work on your Wi-Fi? Does hardware detection report correct RAM on your OS? File an issue with `/api/self` output.
- **Android/Termux**: packaging `shikad` as a Termux:Widget button.

## Ground rules

- Keep the orchestrator **dependency-light**; justify any new Go module in the PR.
- Keep the **control/data plane split**: shikA orchestrates, prima.cpp infers.
- Run `make check` before pushing. Add tests for planner/discovery logic.
- Small, focused PRs with a clear description beat large ones.

## Reporting bugs / ideas

Open an issue. For discovery/cluster bugs, include: your OS mix, `/api/self` from each node, and `/api/plan` from one of them.

By contributing you agree your work is licensed under the project's [MIT License](LICENSE).

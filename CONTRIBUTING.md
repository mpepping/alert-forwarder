# Contributing

1. Fork the repository and create a feature branch off `main`.
2. Make your changes. Keep commits focused and descriptive.
3. Run the local checks before opening a pull request:

   ```bash
   make tidy
   make gofmt
   make vet
   make lint
   make test
   make build
   ```

4. Open a pull request targeting `main`. CI must pass before review.

A clean `go mod tidy` diff, `gofmt`, `go vet`, and `golangci-lint` are required.

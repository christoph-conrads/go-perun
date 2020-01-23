# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0]: Belinda - 2020-2-21
### Added
- Changelog
- Public [Github wiki](https://github.com/perun-network/go-perun/wiki)
- [Godoc](https://godoc.org/perun.network/go-perun)
- [TravisCI](https://travis-ci.org/perun-network)
- [goreportcard](https://goreportcard.com/report/github.com/perun-network/go-perun)
- [codeclimate](https://codeclimate.com/github/perun-network/go-perun)
- Ledger channel dispute
- Ethereum contracts for disputes
  
### Changed
- `Serializable` renamed to `Serializer`
- Unified backend imports
- `pkg/io/test/bytewiteReader` to `iotest.OneByteReader`

### Removed
- Wallet interface
- ethereum/wallet `NewAddressFromBytes`
- `channel/machine` subscription logic

### Fixed
- ? Spelling mistakes
- Cyclomatic simplifications
- Deadlock in Two-party payment test
- `TestSettler_MultipleSettles` timeout

## [0.1.0]: Ariel - 2019-12-20
### Added
- Two-party ledger state channels
- Cooperatively settling two-party ledger channels

[Unreleased]: https://github.com/perun-network/go-perun/compare/v0.2.0...HEAD
[0.1.0]: https://github.com/perun-network/go-perun/releases/tag/v0.1.0
[0.2.0]: https://github.com/perun-network/go-perun/releases/tag/v0.2.0
# Changelog

## [2.1.1](https://github.com/kevholmes/elyclover.com-infra/compare/v2.1.0...v2.1.1) (2025-11-22)


### Bug Fixes

* gh pat expiry ([#169](https://github.com/kevholmes/elyclover.com-infra/issues/169)) ([64b1afc](https://github.com/kevholmes/elyclover.com-infra/commit/64b1afcc82eb62a71c55a33e9ea62ba8b0dc6723))

## [2.1.0](https://github.com/kevholmes/elyclover.com-infra/compare/v2.0.0...v2.1.0) (2023-12-18)


### Features

* add pulumi preview ci test matrix for all stack/envs ([#83](https://github.com/kevholmes/elyclover.com-infra/issues/83)) ([5a863ab](https://github.com/kevholmes/elyclover.com-infra/commit/5a863ab12bcb3cf135228232496b6d668d7e7513))


### Bug Fixes

* only decrypt pfx for prod testflow ([#88](https://github.com/kevholmes/elyclover.com-infra/issues/88)) ([160053c](https://github.com/kevholmes/elyclover.com-infra/commit/160053c35b43cc77a6deb6c6beb6078c889ef0ea))

## [2.0.0](https://github.com/kevholmes/elyclover.com-infra/compare/v1.3.1...v2.0.0) (2023-12-06)


### ⚠ BREAKING CHANGES

* common structs, readability ([#79](https://github.com/kevholmes/elyclover.com-infra/issues/79))

### Bug Fixes

* common structs, readability ([#79](https://github.com/kevholmes/elyclover.com-infra/issues/79)) ([05b7b50](https://github.com/kevholmes/elyclover.com-infra/commit/05b7b50aa980ae8595b184e81f5dae9d83db5dd9))

## [1.3.1](https://github.com/kevholmes/elyclover.com-infra/compare/v1.3.0...v1.3.1) (2023-11-16)


### Bug Fixes

* ci lint azuread lib deprecation, v3 requires switch to clientid var ([#72](https://github.com/kevholmes/elyclover.com-infra/issues/72)) ([b82b588](https://github.com/kevholmes/elyclover.com-infra/commit/b82b588eded518039666595a744a33db1d3dad55))
* dependabot pr grouping for minor/patch version bumps of go libs ([#63](https://github.com/kevholmes/elyclover.com-infra/issues/63)) ([8733436](https://github.com/kevholmes/elyclover.com-infra/commit/8733436ab3ea98d8b72825079d7651c265f55f96))

## [1.3.0](https://github.com/kevholmes/elyclover.com-infra/compare/v1.2.1...v1.3.0) (2023-10-07)


### Features

* migrate to elyclover.com ([#34](https://github.com/kevholmes/elyclover.com-infra/issues/34)) ([87a04a2](https://github.com/kevholmes/elyclover.com-infra/commit/87a04a21f42cecb3efd04156ab68731b660983fb))

## [1.2.1](https://github.com/kevholmes/elyclover.com-infra/compare/v1.2.0...v1.2.1) (2023-09-29)


### Bug Fixes

* allow dependabot access to dev azure SP secret token ([#24](https://github.com/kevholmes/elyclover.com-infra/issues/24)) ([eb29d2f](https://github.com/kevholmes/elyclover.com-infra/commit/eb29d2fcbef6243b590020ae35ff6a4a02e9c6b3))
* flaky pulumi resource diffs on cdn endpoint custom domain resource ([#27](https://github.com/kevholmes/elyclover.com-infra/issues/27)) ([b61aca0](https://github.com/kevholmes/elyclover.com-infra/commit/b61aca0b7f0c86fad2742428626fe605b09d6552))

## [1.2.0](https://github.com/kevholmes/elyclover.com-infra/compare/v1.1.1...v1.2.0) (2023-09-26)


### Features

* add ci checks ([#23](https://github.com/kevholmes/elyclover.com-infra/issues/23)) ([84c38bc](https://github.com/kevholmes/elyclover.com-infra/commit/84c38bc911a79dc9f7002b9a0c0c569c35de66e2))
* import apex domain pfx-type cert and utilize for azure cdn custom domain ep ([#21](https://github.com/kevholmes/elyclover.com-infra/issues/21)) ([b1f4041](https://github.com/kevholmes/elyclover.com-infra/commit/b1f40413ded72467eb6c798f08c8fd5f6f28e48e))

## [1.1.1](https://github.com/kevholmes/elyclover.com-infra/compare/v1.1.0...v1.1.1) (2023-09-25)


### Bug Fixes

* conditional tls setup for prod/nonprod ([#12](https://github.com/kevholmes/elyclover.com-infra/issues/12)) ([81ae0a5](https://github.com/kevholmes/elyclover.com-infra/commit/81ae0a52df238a76012df7a6edd3089fffaba74a))

## [1.1.0](https://github.com/kevholmes/elyclover.com-infra/compare/v1.0.0...v1.1.0) (2023-09-23)


### Features

* automate service principal creation, export to github repo var/secrets for ci/cd workflows ([#10](https://github.com/kevholmes/elyclover.com-infra/issues/10)) ([269e1f7](https://github.com/kevholmes/elyclover.com-infra/commit/269e1f73349a17ade88f0bc69415a70f9277329d))

## 1.0.0 (2023-09-21)


### Features

* add pre-commit config for ggshield secrets detection ([#8](https://github.com/kevholmes/elyclover.com-infra/issues/8)) ([2d4cabe](https://github.com/kevholmes/elyclover.com-infra/commit/2d4cabeff607e0348be21cca07fe9e45ef6a6b23))
* **ci:** add release automation gh action ([#4](https://github.com/kevholmes/elyclover.com-infra/issues/4)) ([0fe23f5](https://github.com/kevholmes/elyclover.com-infra/commit/0fe23f58a5fef627e040406f9b71ffe4e43bc227))
* pulumi implementation of site hosting with multiple envs ([#3](https://github.com/kevholmes/elyclover.com-infra/issues/3)) ([a413842](https://github.com/kevholmes/elyclover.com-infra/commit/a4138424979e301908091addb2808aac6bf72fb8))

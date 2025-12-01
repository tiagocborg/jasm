## 1.0.0 (2025-12-01)

### ⚠ BREAKING CHANGES

* indicators and footers

Commit Types:
- feat: → minor version
- fix: → patch version
- feat!: → major version (breaking change)
- docs, test, ci, chore: → no release

Benefits:
- No manual release process needed
- Automatic versioning based on actual changes
- Changelog always in sync with releases
- Multi-architecture Docker images automatically built
- Clear commit history and semantic meaning
- Enforces code quality before release

The GITHUB_TOKEN secret (already available) is used for:
- Creating tags and releases
- Pushing to main branch for changelog updates
- Publishing GitHub Releases
* indicators and footers

Commit Types:
- feat: → minor version
- fix: → patch version
- feat!: → major version (breaking change)
- docs, test, ci, chore: → no release

Benefits:
- No manual release process needed
- Automatic versioning based on actual changes
- Changelog always in sync with releases
- Multi-architecture Docker images automatically built
- Clear commit history and semantic meaning
- Enforces code quality before release

The GITHUB_TOKEN secret (already available) is used for:
- Creating tags and releases
- Pushing to main branch for changelog updates
- Publishing GitHub Releases

### Features

* add 1Password secret provider with OTLP metrics ([795010f](https://github.com/tiagocborg/jasm/commit/795010fc80d761a9d54523d330f1e4142f8ca168))

### Bug Fixes

* add missing conventional-changelog dependency to release workflow ([4e12d49](https://github.com/tiagocborg/jasm/commit/4e12d49345cde8b7d7558c562d21a79cd0570e9f))
* add MIT license file ([25081e1](https://github.com/tiagocborg/jasm/commit/25081e1b722d81352bab03ea48c9d6ad6ff951e6))
* correct docker workflow syntax error ([6011252](https://github.com/tiagocborg/jasm/commit/601125201b469b4532698675f648c06ed4e3744a))
* correct semantic-release labels configuration ([81938dc](https://github.com/tiagocborg/jasm/commit/81938dc6c2314f0c8217bd8f4156370fa1533113))
* fetch tags before docker build in release workflow ([e9287f9](https://github.com/tiagocborg/jasm/commit/e9287f9ebc74cb6395ba6aa25196f934d575fa5f))
* **provider:** handle numeric and non-string JSON values in AWS secrets ([1964b7b](https://github.com/tiagocborg/jasm/commit/1964b7b449ab0e115929f7695132e4feadb9bb72))

### Continuous Integration

* Add comprehensive GitHub Actions CI/CD with semantic-release ([3a03314](https://github.com/tiagocborg/jasm/commit/3a03314be369674127a3f8f6ac9dcd0c42dd962f))
* Add comprehensive GitHub Actions CI/CD with semantic-release ([1e34643](https://github.com/tiagocborg/jasm/commit/1e34643922ba33f50fba417502fa1ba51bc14b79))

## 1.0.0 (2025-11-30)

### ⚠ BREAKING CHANGES

* indicators and footers

Commit Types:
- feat: → minor version
- fix: → patch version
- feat!: → major version (breaking change)
- docs, test, ci, chore: → no release

Benefits:
- No manual release process needed
- Automatic versioning based on actual changes
- Changelog always in sync with releases
- Multi-architecture Docker images automatically built
- Clear commit history and semantic meaning
- Enforces code quality before release

The GITHUB_TOKEN secret (already available) is used for:
- Creating tags and releases
- Pushing to main branch for changelog updates
- Publishing GitHub Releases
* indicators and footers

Commit Types:
- feat: → minor version
- fix: → patch version
- feat!: → major version (breaking change)
- docs, test, ci, chore: → no release

Benefits:
- No manual release process needed
- Automatic versioning based on actual changes
- Changelog always in sync with releases
- Multi-architecture Docker images automatically built
- Clear commit history and semantic meaning
- Enforces code quality before release

The GITHUB_TOKEN secret (already available) is used for:
- Creating tags and releases
- Pushing to main branch for changelog updates
- Publishing GitHub Releases

### Features

* add 1Password secret provider with OTLP metrics ([6448385](https://github.com/tiagocborg/jasm/commit/6448385d4853430ce7457cbd6072e40ecd65feea))

### Bug Fixes

* add missing conventional-changelog dependency to release workflow ([54cd07d](https://github.com/tiagocborg/jasm/commit/54cd07dc958c63f8910c43b1dd8daa3100f38263))
* add MIT license file ([25081e1](https://github.com/tiagocborg/jasm/commit/25081e1b722d81352bab03ea48c9d6ad6ff951e6))
* correct docker workflow syntax error ([cfaedbc](https://github.com/tiagocborg/jasm/commit/cfaedbcd43e3b0f23a6f71cb4fa6ac5c92428896))
* correct semantic-release labels configuration ([176da23](https://github.com/tiagocborg/jasm/commit/176da2356846357669aa71cf10896844be3a3167))
* **provider:** handle numeric and non-string JSON values in AWS secrets ([1964b7b](https://github.com/tiagocborg/jasm/commit/1964b7b449ab0e115929f7695132e4feadb9bb72))

### Continuous Integration

* Add comprehensive GitHub Actions CI/CD with semantic-release ([3a03314](https://github.com/tiagocborg/jasm/commit/3a03314be369674127a3f8f6ac9dcd0c42dd962f))
* Add comprehensive GitHub Actions CI/CD with semantic-release ([1e34643](https://github.com/tiagocborg/jasm/commit/1e34643922ba33f50fba417502fa1ba51bc14b79))

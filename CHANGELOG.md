# Changelog

## [1.7.1](https://github.com/kagento/kagento-cli/compare/v1.7.0...v1.7.1) (2026-03-30)


### Bug Fixes

* make image mirroring best-effort during vcluster task publish ([30876ce](https://github.com/kagento/kagento-cli/commit/30876cec43148da761e9dcc0b240d6650be6e6aa))

## [1.7.0](https://github.com/kagento/kagento-cli/compare/v1.6.3...v1.7.0) (2026-03-30)


### Features

* mirror task images to scoped registry during vcluster task submit ([c0aeb00](https://github.com/kagento/kagento-cli/commit/c0aeb0096ea4485bf000ce1b6eb228a860c25cda))

## [1.6.3](https://github.com/kagento/kagento-cli/compare/v1.6.2...v1.6.3) (2026-03-30)


### Bug Fixes

* check command waits for 'running' status to complete ([5b08d4f](https://github.com/kagento/kagento-cli/commit/5b08d4fabaedf8b52093442500dbed7ea1d750e0))

## [1.6.2](https://github.com/kagento/kagento-cli/compare/v1.6.1...v1.6.2) (2026-03-28)


### Bug Fixes

* route FinishSession through backend API instead of Supabase ([ef5ff4e](https://github.com/kagento/kagento-cli/commit/ef5ff4ec7d072f89f96ebbd38b933e80c49b85fc))

## [1.6.1](https://github.com/kagento/kagento-cli/compare/v1.6.0...v1.6.1) (2026-03-28)


### Bug Fixes

* allow PodDisruptionBudget in vcluster task manifests ([50663cd](https://github.com/kagento/kagento-cli/commit/50663cd5b2c4c6e9a3790568cb74e9190df9848e))

## [1.6.0](https://github.com/kagento/kagento-cli/compare/v1.5.0...v1.6.0) (2026-03-28)


### Features

* add `kagento start` command to create sessions from CLI ([3d48bb9](https://github.com/kagento/kagento-cli/commit/3d48bb9df1724b52bee7b3bde71bc1b841d4acb9))

## [1.5.0](https://github.com/kagento/kagento-cli/compare/v1.4.1...v1.5.0) (2026-03-26)


### Features

* add build inspection and control commands for task authors
* add authored task get and update commands
* add batch task submit and publish flows with bounded parallelism
* add JSON output to more commands and add CLI self-update

## [1.4.1](https://github.com/kagento/kagento-cli/compare/v1.4.0...v1.4.1) (2026-03-26)


### Bug Fixes

* refresh saved auth tokens during long-running task submit and publish flows
* retry transient build status failures and auto-discover task metadata for publish

## [1.4.0](https://github.com/kagento/kagento-cli/compare/v1.3.0...v1.4.0) (2026-03-25)


### Features

* accept session names (two-word) in all commands ([dc27627](https://github.com/kagento/kagento-cli/commit/dc27627159c740730b87ee4f73f88079dddc69cd))

## [1.3.0](https://github.com/kagento/kagento-cli/compare/v1.2.0...v1.3.0) (2026-03-25)


### Features

* use Supabase for all CLI operations, remove backend dependency ([0b3d671](https://github.com/kagento/kagento-cli/commit/0b3d671710485ac645d3458e2560217d0b92553a))

## [1.2.0](https://github.com/kagento/kagento-cli/compare/v1.1.1...v1.2.0) (2026-03-25)


### Features

* migrate CLI to Supabase PostgREST and remove Hasura ([c75fd21](https://github.com/kagento/kagento-cli/commit/c75fd217008cf49f37fd534f5611e0aafb79fe33))

## [1.1.1](https://github.com/kagento/kagento-cli/compare/v1.1.0...v1.1.1) (2026-03-25)


### Bug Fixes

* default backend URL to production instead of localhost ([88c2cc0](https://github.com/kagento/kagento-cli/commit/88c2cc004a0033be44baa1de2f769b3867da398b))

## [1.1.0](https://github.com/kagento/kagento-cli/compare/v1.0.0...v1.1.0) (2026-03-25)


### Features

* add comprehensive task validation ([4ace67c](https://github.com/kagento/kagento-cli/commit/4ace67c6ef21522fca6e5b194f89c9e62afd98a3))
* add kubeconfig download command ([ad81248](https://github.com/kagento/kagento-cli/commit/ad812485bc1d48ccdbfde40a9f6ef16e2abb105d))
* add task authoring CLI commands ([d6a3355](https://github.com/kagento/kagento-cli/commit/d6a3355a85493e75bd6f14823a73e03f02c45bbe))
* auto-login to registry on task publish ([f179d6e](https://github.com/kagento/kagento-cli/commit/f179d6e9044b39995212a05f43a6c45888426a71))
* migrate auth to Supabase and add vcluster task support ([636fee5](https://github.com/kagento/kagento-cli/commit/636fee57efa5b3ea809b9ddc8feedb7bdb25a942))


### Bug Fixes

* disallow TASK.md in task directories and Dockerfiles ([4cf40c5](https://github.com/kagento/kagento-cli/commit/4cf40c5c78cdc446ac7f230d24703ba5e50a2547))

## 1.0.0 (2026-03-20)


### Features

* add automated CLI release pipeline ([#1](https://github.com/kagento/kagento-cli/issues/1)) ([cf5125d](https://github.com/kagento/kagento-cli/commit/cf5125d238d312e71e155f97a1831478c542db81))

## Changelog

All notable changes to this project will be documented in this file.

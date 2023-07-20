# Release Process

The cluster-proportional-vertical-autoscaler (cpvpa) is released on an as-needed basis. The process is as follows:

1. Someone must file an issue proposing a new release with a changelog since the last release.
1. All [OWNERS](OWNERS) must LGTM this release.
1. An OWNER (who must have push access to the `registry.k8s.io/cpa` project):
    1. Tags the commit approved for release with `git tag -s vx.x.x`. The `vx.x.x` is semver with a leading `v`.
    1. Runs `make push`, to build and push the container image for the release to `registry.k8s.io/cpa`.
    1. Creates a [github release](https://github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler/releases/new) with a message pointing to the pushed container image.

1. The release issue is closed.
1. An announcement email is sent to `kubernetes-dev@googlegroups.com` with the subject `[ANNOUNCE] cluster-proportional-vertical-autoscaler vx.x.x is released`.

Example:

```
$ git tag
v0.2.0
v0.2.1
v0.3.1
v0.4.0
v1.0.0
v2.0.0

# Pick the new release number

$ git tag -am "v2.0.1" v2.0.1

$ make container
<...lots of output...>
container: registry.k8s.io/cpa/cpvpa-amd64:v2.0.1

$ gcloud docker -- push registry.k8s.io/cpa/cpvpa-amd64:v2.0.1
<...lots of output...>
v2.0.1: digest: sha256:504833aedf3f14379e73296240ed44d54aecd4c02367b004452dfeca2465e5bf size: 950

# Create the github release for v2.0.1, with body "registry.k8s.io/cpa/cpvpa-amd64:v2.0.1".
```

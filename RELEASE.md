# Release Process

The cluster-proportional-vertical-autoscaler (cpvpa) is released on an as-needed basis. The process is as follows:

1. Someone must file an issue proposing a new release with a changelog since the last release.
1. All [OWNERS](OWNERS) must LGTM this release.
1. An OWNER (who must have push access to the `registry.k8s.io/cpa` project):
    1. Tags the commit approved for release with `git tag -s vx.x.x`. The `vx.x.x` is semver with a leading `v`.
    1. Runs `make push`, to build and push the container image for the release to `registry.k8s.io/cpa`.
    1. Creates a [github release](https://github.com/kubernetes-sigs/cluster-proportional-vertical-autoscaler/releases/new) with a message pointing to the pushed container image.

1. The release issue is closed.
1. An announcement email is sent to `dev@kubernetes.io` with the subject `[ANNOUNCE] cluster-proportional-vertical-autoscaler vx.x.x is released`.

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

$ make manifest-list
<...lots of output...>
Digest: sha256:abd3f14ed2b091006ba81b3f3dc191f7400d64c7fb0765699f92d5d008df0c33 1556

# Promote from k8s-staging-cpa to registry.k8s.io/cpa.

# Create the github release for v2.0.1, with body "registry.k8s.io/cpa/cpvpa:v2.0.1".
```

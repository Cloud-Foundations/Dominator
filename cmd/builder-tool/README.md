# builder-tool
A utility to control the *[imaginator](../imaginator/README.md)* image building service.

The *builder-tool* utility may be used to build images from manifests, either
locally (for advanced debugging) or by sending a build request to the
*[imaginator](../imaginator/README.md)* image building service.

## Usage
*Builder-tool* supports several sub-commands. There are many command-line flags
which provide parameters for these sub-commands. The most commonly used
parameter is `-imaginatorHostname` which specifies which host the *imaginator*
is running on. The `-alwaysShowBuildLog` parameter is also commonly used.
The basic usage pattern is:

```
builder-tool [flags...] command [args...]
```

Built-in help is available with the command:

```
builder-tool -h
```

Some of the sub-commands available are:

- **build-from-manifest**: build an image locally from the specified manifest
                           directory and upload a new datestamped image for the
                           specified image stream name. The source image will be
                           fetched from the *[imageserver](../imageserver/README.md)*
- **build-image**: request the *[imaginator](../imaginator/README.md)* to build
                   and upload an image for the specified image stream (this is
                   the most commonly-used sub-command)
- **build-raw-from-manifest**: build an image locally from the specified
                               manifest and write a RAW image file which can be
                               copied/uploaded for launching VMs. The source
                               image will be fetched from the *[imageserver](../imageserver/README.md)*
- **build-tree-from-manifest**: build an image file-system tree locally from the
                                specified manifest directory. The root directory
                                for the tree is created and written to stdout.
                                The source image will be fetched from the
                                *[imageserver](../imageserver/README.md)*
- **disable-auto-builds**: disable automatic image building for the period
                           specified by `-disableFor`
- **disable-build-requests**: disable automatic image building for the period
                              specified by `-disableFor`
- **get-dependencies**: get the dependencies for all the *[imaginator](../imaginator/README.md)*
                        image streams and write a JSON representation to stdout
- **get-digraph**: get the image stream dependencies represented as a directed
                   graph suitable for passing to the *dot* command from the
                   *Graphviz* tools
- **process-manifest**: process a manifest locally in the specified root
                        directory containing an already unpacked source image
- **replace-idle-slaves**: replace build slaves which are idle

## Security
*[Imaginator](../imaginator/README.md)* restricts RPC access using TLS client
authentication. *Builder-tool* will load certificate and key files from the
`~/.ssl` directory. *Builder-tool* will present these certificates to the
*imaginator*. If one of the certificates is signed by a certificate authority
that the *imaginator* trusts, the *imaginator* will grant access.

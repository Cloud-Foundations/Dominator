# objtool
A utility to add, check, delete and get objects on the
*[objectserver](../imageserver/README.md)*.

The *objtool* is the most important utility in the **Dominator** system, as it
is used to manage images. *Objtool* may be run on any machine. It is typically
run on a desktop, bastion or build machine, depending on the sophistication of
your build environment.

## Usage
*Objtool* supports several sub-commands. There are many command-line flags
which provide parameters for these sub-commands. The most commonly used
parameter is `-objectServerHostname` which specifies which host the
*objectserver* to talk to is running on. The basic usage pattern is:

```
objtool [flags...] command [args...]
```

Built-in help is available with the command:

```
objtool -h
```

Some of the sub-commands available are:

- **add**: add multiple files (objects) specified to the subcommand
- **check**: check if an object exists
- **get**: get a specified object
- **import**: import a list of objects from a remote (HTTP/HTTPS) server
- **mget**: get a list of objects
- **get-build-log**: get build log for an image
- **test-bandwidth-from-server**: test the speed for downloading from the server
- **test-bandwidth-to-server**: test the speed for uploading to the server

## Security
*[Objectserver](../imageserver/README.md)* restricts RPC access using TLS client
authentication. *Objtool* will load certificate and key files from the
`~/.ssl` directory. *Objtool* will present these certificates to
*objectserver*. If one of the certificates is signed by a certificate authority
that *objectserver* trusts, *objectserver* will grant access.

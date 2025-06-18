# image-unpacker
The *ImageUnpacker* daemon can be used to unpack *[imageserver](../imageserver/README.md)* images into partitions or external volumes.

The *image-unpacker* daemon is used by the
*[AMI Publisher](../ami-publisher/README.md)* to unpack images onto external
volumes that can then be used to create AMIs. See the
[design document](../../design-docs/AmiPublisher/README.md) for more
information.

## Status page
The *image-unpacker* provides a web interface on port `6974` which shows a status
page, links to built-in dashboards and access to performance metrics and logs.
If *image-unpacker* is running on host `myhost` then the URL of the main
status page is `http://myhost:6974/`. An RPC over HTTP interface is also
provided over the same port.


## Startup
*image-unpacker* is started at boot time, usually by one of the provided
[init scripts](../../init.d/). The *image-unpacker* process is baby-sat by the init
script; if the process dies the init script will re-start it. It may be stopped
with the command:

```
service image-unpacker stop
```

which also kills the baby-sitting init script. It may be started with the
command:

```
service image-unpacker start
```

There are many command-line flags which may change the behaviour of
*image-unpacker* but many have defaults which should be adequate for most
deployments. Built-in help is available with the command:

```
image-unpacker -h
```

## Security
RPC access is restricted using TLS client authentication. *image-unpacker*
expects a root certificate in the file `/etc/ssl/CA.pem` which it trusts to sign
certificates which grant access to methods. It trusts the root certificate in
the `/etc/ssl/IdentityCA.pem` file to sign identity-only certificates.

It also requires a certificate and key which grant it the ability to fetch
images and objects from an *[imageserver](../imageserver/README.md)*. These
should be in the files `/etc/ssl/image-unpacker/cert.pem` and
`/etc/ssl/image-unpacker/key.pem`, respectively.

## Control
The *[unpacker-tool](../unpacker-tool/README.md)* utility may be used to manage
the service. Normally the *[AMI Publisher](../ami-publisher/README.md)* is used
for higher-level image publication and management.

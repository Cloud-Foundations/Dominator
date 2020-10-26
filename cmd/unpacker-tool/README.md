# unpacker-tool
A utility to control the *image-unpacker*.

The *unpacker-tool* utility may be used to debug and control a running
*image-unpacker*.
*Unpacker-Tool* may be run on any machine and can be used to issue the low-level
RPC requests used in building bootable image artefacts. It is typically run on a
desktop or bastion machine.

## Usage
*Unpacker-tool* supports several sub-commands. There are many command-line flags
which provide parameters for these sub-commands. The most commonly used
parameter is `-imageUnpackerHostname` which specifies which host the
*image-unpacker* to talk to is running on. The basic usage pattern is:

```
unpacker-tool [flags...] command [args...]
```

Built-in help is available with the command:

```
unpacker-tool -h
```

Some of the sub-commands available are:

- **add-device**: add a device (block storage volume) with specified external
                  identifier. The command will be executed to dynamically add
                  (attach) the storage volume to the machine running the
                  *image-unpacker*
- **associate**: associate an image stream with the specified device
- **claim-device**: claim (register) an existing device
- **export-image**: export image to a specified destination (i.e. S3-backed AMI)
- **get-raw**: get the raw contents of the device storing the image and write to
               the local file specified by the `-filename` parameter
- **get-status**: get status information for the *image-unpacker*
- **get-device-for-stream**: get the device ID for the specified image stream
- **prepare-for-capture**: prepare a previously unpacked image for capture by
                           adding/updating a bootloader
- **prepare-for-copy**: prepare the device for copying the contents from the
                        *image-unpacker* (i.e. `scp`) or with **get-raw**
- **prepare-for-unpack**: prepare the device for unpacking an image by scanning
                          the file-system
- **remove-device**: remove an unused device from management
- **unpack-image**: unpack the specfied image version (leaf name) for the stream

## Security
*Image-unpacker* restricts RPC access using TLS client authentication.
*Unpacker-tool* will load certificate and key files from the `~/.ssl`
directory. *Unpacker-tool* will present these certificates to *image-unpacker*.
If one of the certificates is signed by a certificate authority
that *image-unpacker* trusts, *image-unpacker* will grant access.

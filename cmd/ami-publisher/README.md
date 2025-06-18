# ami-publisher
A utility to manage and publish Amazon Machine Images (AMIs) using images stored
in the *[imageserver](../imageserver/README.md)*, using *[ImageUnpacker](../image-unpacker/README.md)* instances.

The *ami-publisher* allows you to efficiently create (publish) AMIs in multiple
AWS accounts and regions. It uses *[ImageUnpacker](../image-unpacker/README.md)* instances running in AWS to
effeciently fetch images from the *[imageserver](../imageserver/README.md)* and
then issues AWS API calls to create the AMIs. It may be run on any machine which
has access to AWS credentials. It is typically run from a script which may be
part of an image build pipeline.

See the [design document](../../design-docs/AmiPublisher/README.md) for more information.

## Usage
*Ami-publisher* supports several sub-commands. There are many command-line flags
which provide parameters for these sub-commands. The most commonly used
parameter is `-imageServerHostname` which specifies which host the *imageserver*
to talk to is running on. The basic usage pattern is:

```
ami-publisher [flags...] command [args...]
```

Built-in help is available with the command:

```
ami-publisher -h
```

Some of the sub-commands available are:

- **add-volumes**: add a volume of the specified size (in GiB) to the
                   *[ImageUnpacker](../image-unpacker/README.md)* instances.
                   This is mostly for debugging
- **copy-bootstrap-image**: will copy images of the named stream between targets
                            (account,region tuples), attempting to copy from the
                            closest region for each destination. It creates
                            temporary instances which run copy commands. This is
                            used to bootstrap the
                            *[ImageUnpacker](../image-unpacker/README.md)*
                            images
- **delete**: delete the image resources specified in the results files
- **delete-tags**: delete the specified tag from resources listed in the results
                   files
- **delete-tags-on-unpackers**: delete the specified tag from
                                *[ImageUnpacker](../image-unpacker/README.md)*
                                instances
- **delete-unused-images**: delete images which are not used by instances, using
                            the exclude and search tags and the list of targets
- **expire**: delete resources which have expired
- **import-key-pair**: import the specified SSH public key into the targets
- **launch-instances**: create (launch) instances in the targets using the
                        specified image and write the created instances to the
                        specified results file
- **launch-instances-for-images**: create (launch) instances for images
                                   specified in the results files
- **list-images**: list images in the specified targets using search and exclude
                   tags
- **list-streams**: list the image streams for all the
                    *[ImageUnpacker](../image-unpacker/README.md)* instances in
                    the specified targets
- **list-unpackers**: list the *[ImageUnpacker](../image-unpacker/README.md)*
                      instances in the specified targets
- **list-unused-images**: list unused images in the specified targets using
                          search and exclude tags
- **list-used-images**: list used images in the specified targets using
                          search and exclude tags
- **prepare-unpackers**: prepare *[ImageUnpacker](../image-unpacker/README.md)*
                         instances in the specified targets for use. Stopped
                         instances will be started and the
                         *[ImageUnpacker](../image-unpacker/README.md)* service
                         will be waited for. If an image stream name is
                         specified, scanning is started
- **publish**: publish AMIs in the specified targets for the specified image
               stream and the leaf name (version). This will create and attach
               volumes if needed
- **remove-unused-volumes**: remove volumes not associated with image streams
                             on the
                             *[ImageUnpacker](../image-unpacker/README.md)*
                             instances in the targets
- **set-exclusive-tags**: set the specified tag for the AMIs in the specified
                          results files and delete the tag key from other AMIs
- **set-tags-on-unpackers**: set the specified tags on the
                             *[ImageUnpacker](../image-unpacker/README.md)*
                             instances in the specified targets
- **start-instances**: start the *[ImageUnpacker](../image-unpacker/README.md)*
                       instances in the specified targets
- **stop-idle-unpackers**: stop *[ImageUnpacker](../image-unpacker/README.md)*
                           instances in the specified targets which have been
                           idle for the specified time
- **terminate-instances**: terminate the
                           *[ImageUnpacker](../image-unpacker/README.md)*
                           instances in the specified targets

## Security
*[Imageserver](../imageserver/README.md)* restricts RPC access using TLS client
authentication. *Ami-Publisher* will load certificate and key files from the
`~/.ssl` directory. *Ami-Publisher* will present these certificates to
*imageserver*. If one of the certificates is signed by a certificate authority
that *imageserver* trusts, *imageserver* will grant access.

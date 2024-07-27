# installer
The *installer* will install an OS on a machine using a **Dominator** image.

The *installer* is a command-line tool that will configure the network, fetch
configuration data, format storage devices, install an OS image and then fast
reboot into the newly installed OS. It is usually packaged into an *installer
image* which may be network booted (via a PXE DHCP/TFTP server) or on portable
storage media such as a USB flash drive or a CD-ROM containg an ISO-format
image. Some machines have a BMC (Baseboard Management Controller) which supports
network mounting a remote ISO image and booting from that.

The *installer* will scan the storage devices for a previously installed OS. If
present, it will copy files from the old OS into a temporary object cache,
computing checksums of the files. It uses this cache to skip downloading objects
from the *[imageserver](../imageserver/README.md)*, saving download time and
network resources.

The *installer* is one component in
[birthing machines](../../design-docs/MachineBirthing/README.md).

## Usage
By default, the *installer* will issue a DHCP request to discover the IP address
for the machine and the address of a TFTP server from where it will fetch
configuration data. If the `config.json` machine configuration file is already
present, the configuration data are not fetched via TFTP. This mode supports
iteratively writing the configuration data and testing the *installer* code
itself.

Built-in help is available with the command:

```
installer -h
```

When network booting, the *[hyper-control](../hyper-control/README.md)* tool may
be used to request that a nearby *[Hypervisor](../hypervisor/README.md)* serve
as a PXE server for the machine.

## Status page
The *installer* provides a web interface on port `6978` which provides a
status page, links to built-in dashboards and access to performance metrics and
logs. If the *installer* is running on host `myhost` then the URL of the main
status page is `http://myhost:6978/`. An RPC over HTTP interface is also
provided over the same port. This port is only open while the machine is being
installed. Once the machine starts booting into the new OS, the port will no
longer be available.

## Remote shell
The *installer* provides a remote shell (over RPC) interface which may be used
for debugging, repairing failing installations and watching the progress of the
installation. The *[hyper-control](../hyper-control/README.md)* utility can be
used to connect to the remote shell interface, using this command:
```
hyper-control installer-shell $hostname
```
where `$hostname` is the name/address of the machine running the *installer*.

## Security
RPC access is restricted using TLS client authentication. The *installer*
expects a root certificate in the file `/etc/ssl/CA.pem` which it trusts to sign
certificates which grant access. It also requires a certificate and key which
clients will use to validate the server. These should be in the files
`/etc/ssl/installer/cert.pem` and `/etc/ssl/installer/key.pem`,
respectively.

## Configuration data
The following files are fetched from the TFTP server (or must already be
present):
- `config.json`: the machine configuration. The schema is defined in the
  [GetMachineInfoResponse](https://github.com/Cloud-Foundations/Dominator/blob/master/proto/fleetmanager/messages.go) type

- `imagename`: the name of the image to fetch from the *[imageserver](../imageserver/README.md)*

- `imageserver`: the address of the *[imageserver](../imageserver/README.md)*

- `storage-layout.json`: the desired configuration of the storage devices. The
  schema is defined in the
  [StorageLayout](https://github.com/Cloud-Foundations/Dominator/blob/master/proto/installer/messages.go) type

## Sequence
The following sections describe the sequence of operations that the *installer*
performs.

### Update system time
Since some systems have an out-of-date clock (perhaps years in the past), the
`/build-timestamp` file modification time is checked, and if the system clock is
behind that time, the time is advanced to the timestamp file. This ensures that
the system clock is at least no older than the time that the installer image was
built.

### Configure local network
The broadcast (EtherNet) interfaces are discovered and turned on. This gives
some time for the links to become stable before active link detection is
performed.

If the `config.json` file is not present, a DHCP request is issued to discover
the IP address for the machine and the address of a TFTP server from where it
will fetch configuration data. The primary network interface is configured based
on the DHCP response. The configuration files are downloaded from the TFTP
server.

### Configure storage
The storage devices are discovered and the image (without objects) is downloaded
from the *[imageserver](../imageserver/README.md)*. An encryption key is
generated.

The objects in the booted OS (typically netbooted) are scanned and checksummed
and added to the object cache. The bulk of these will typically be the kernel
image and modules.

The boot device is scanned for an OS and if present is scanned and files are
checksummed and added to the object cache.

The OS image is unpacked into a tmpfs, downloading objects which are not in the
cache. This ensures that all objects are available prior to modifying the
storage devices. It also provides various utilities required for partitioning,
encrypting and formatting storage devices.

The storage devices are erased (either with `blkdiscard` or by writing 1 MiB of
zeros at the beginning) and then partitioned. Except for the partition which
will contain the new root file-system, they are by default encrypted.
File-systems are created and the OS is installed on the root device. These
operations are performed concurrently as these are typically I/O bound
operations.

Mount points and entries in `/etc/fstab` are created for the non-root
file-systems. The encryption key is written to the root file-system. The
configuration files are written to the `/var/log/installer` directory.

### Configure network
The broadcast interfaces are checked to see which have an active link (i.e.
connected to an active switch port). The machine configuration is consulted and
a network configuration file is rendered and written to the new root
file-system. The configuration file specifies interfaces, trunks, VLANs and
bridges.

### Copy installation logs
The installation logs are written to `/var/log/installer/log`.

### Unmount storage
The storage devices are unmounted.

### Reboot
If the `-useKexec=true` option was passed to
*[hyper-control](../hyper-control/README.md)* when requesting the network boot,
the `kexec` utility is used to reboot into the new OS. Otherwise, the normal
`reboot` system call is used, which causes a machine restart and a BIOS boot.

## Example Configuration files

### Machine configuration
An example `config.json`:
```
{
    "Machine": {
        "Hostname": "hyper0",
        "HostIpAddress": "192.168.1.2",
        "HostMacAddress": "de:ad:be:ef:00:01",
        "IPMI": {
            "Hostname": "hyper0-ipmi",
            "HostIpAddress": "192.168.0.2",
            "HostMacAddress": "de:ad:be:ef:00:00"
        },
        "Tags": {
            "RequiredImage": "hypervisor/2024-01-18:22:57:43",
            "Type": "SuperMicro Ultra"
        }
    },
    "Subnets": [
        {
            "Id": "Dev",
            "IpGateway": "192.168.100.1",
            "IpMask": "255.255.255.0",
            "DomainName": "dev.company.com",
            "DomainNameServers": [
                "192.168.1.53",
                "8.8.8.8"
            ],
            "Manage": true,
            "VlanId": 100
        },
        {
            "Id": "Hypervisors",
            "IpGateway": "192.168.1.1",
            "IpMask": "255.255.255.0",
            "DomainName": "hypervisors.company.com",
            "DomainNameServers": [
                "192.168.1.53",
                "8.8.8.8"
            ]
        },
        {
            "Id": "Prod",
            "IpGateway": "192.168.120.1",
            "IpMask": "255.255.255.0",
            "DomainName": "prod.company.com",
            "DomainNameServers": [
                "192.168.1.53",
                "8.8.8.8"
            ],
            "Manage": true,
            "VlanId": 120
        }
    ]
}
```
The rendered network configuration file will contain an interface with the
primary IP address (`192.168.1.2`) with a default and DNS configuration taken
from the matching subnet declaration (the `Hypervisors` subnet).

The IPMI information is used by the
*[fleet-manager](../fleet-manager/README.md)* and
*[hyper-control](../hyper-control/README.md)* to identify the IPMI
address to use for management operations.

Any EtherNet interfaces with active links will be trunked (if there are two or
more) and VLAN and bridge interfaces added for each subnet declaration which has
`Manage` set to true.

### Storage layout
The default `storage-layout.json` is:
```
{
    "BootDriveLayout": [
        {
            "FileSystemType": "ext4",
            "MountPoint": "/",
            "MinimumFreeBytes": 2147483648
        },
        {
            "FileSystemType": "ext4",
            "MountPoint": "/home",
            "MinimumFreeBytes": 1073741824
        },
        {
            "FileSystemType": "ext4",
            "MountPoint": "/var/log",
            "MinimumFreeBytes": 536870912
        }
    ],
    "ExtraMountPointsBasename": "/data/",
    "Encrypt": true,
    "UseKexec": false
}
```
This will create:
- a root file-system with 2GiB of free space after the OS installation
- a `/home` file-system with 1 GiB free
- a `/var/log` file-system with 512 MiB free
- a `/data/0` file-system consuming the remainder of the boot storage device
- zero or more `/data/#` file-systems for each secondary storage device

All the file-systems except for `/` will be encrypted. The `kexec` reboot method
will not be used.

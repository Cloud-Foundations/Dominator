# vm-control
A utility to manage Virtual Machines (VMs).

The *vm-control* utility creates and manages VMs by communicating with a
*[Hypervisor](../hypervisor/README.md)*. It is typically run on a desktop, bastion or build machine.

## Usage
*vm-control* supports several sub-commands. There are many command-line flags
which provide parameters for these sub-commands. The most commonly used
parameters are `-fleetManagerHostname` or `-hypervisorHostname` which specify
either the *[Fleet Manager](../fleet-manager/README.md)* or a specific *[Hypervisor](../hypervisor/README.md)* to communicate with.
The basic usage pattern is:

```
vm-control [flags...] command [args...]
```

Built-in help is available with the command:

```
vm-control -h
```

Some of the sub-commands available are:

- **add-vm-volumes**: add volumes to a VM
- **become-primary-vm-owner**: become the primary owner of a VM
- **change-vm-console-type**: change the console type for a VM
- **change-vm-cpu-priority**: change the CPU priority for a VM
- **change-vm-cpus**: change the number of CPUs for a VM
- **change-vm-destroy-protection**: enable/disable destroy protect for a VM
- **change-vm-machine-type**: change the machine type for a VM
- **change-vm-memory**: change the memory for a VM
- **change-vm-owner-groups**: change the owner groups for a VM
- **change-vm-owner-users**: change the extra owner users for a VM
- **change-vm-subnet**: change the subnet ID for a VM. The primary IP address
                        will change
- **change-vm-tags**: change the tags for a VM
- **change-vm-vcpus**: change the number of vCPUs for a VM
- **change-vm-volume-interfaces**: change the volume interfaces (device types
                                   presented to VM) for the specified VM
- **change-vm-volume-size**: change the size of a specified volume on a VM
- **connect-to-vm-console**: connect to the Virtual Network Console for the
                             specified VM
- **connect-to-vm-serial-port**: connect to the specified VM serial port
- **copy-vm**: make a copy of a VM. The new VM will have a different IP address
- **create-vm**: create a VM
- **debug-vm-image**: (re)start a VM with a temporary debug image. The old root
                      volume will become the first secondary volume. Debugging
                      ends when the VM is stopped or (re)started
- **delete-vm-volume**: delete a specified volume from a VM
- **destroy-vm**: destroy a VM (all ephemeral data and metadata are lost)
- **discard-vm-old-image**: discard the previous root image for a VM
- **discard-vm-old-user-data**: discard the previous user data for a VM
- **discard-vm-snapshot**: discard the previous snapshot for a VM
- **export-local-vm**: export a local VM to an importing tool. This is primarily
                       for debugging
- **export-virsh-vm**: export VM to a local virsh VM. The specified FQDN will
                       be used to specify the new virsh domain name. The VM
                       must first be stopped. The exported virsh VM is started
- **get-hypervisors**: get details of healthy Hypervisors in the specified
                       location
- **get-vm-hypervisor**: get and show the *Hypervisor* for a VM
- **get-vm-info**: get and show the information for a VM
- **get-vm-infos**: get and show the information for all VMs on a *Hypervisor*
- **get-vm-user-data**: get (copy) the user data for a VM
- **get-vm-volume**: get (copy) a specified VM volume
- **import-local-vm**: import a local raw VM. This is primarily for debugging
- **import-virsh-vm**: import a local virsh VM. The specified domain name must
                       be a FQDN, which is used to obtain the IP address of the
                       imported VM. The virsh VM must first be shut down. The
                       imported VM is started
- **list-hypervisors**: list healthy Hypervisors in the specified location
- **list-locations**: list locations within the specified top location
- **list-vms**: list the IP addresses for all VMs
- **migrate-vm**: migrate a VM to another Hypervisor
- **parse-virsh-xml**: parse the XML for a virsh VM
- **patch-vm-image**: patch the root image for a VM. Files listed in the image
                      filter are not changed. The old root image is saved. The
                      VM must not be running
- **probe-vm-port**: probe (from its *Hypervisor*) a TCP port for a VM
- **reboot-vm**: reboot a VM
- **reorder-vm-volumes**: re-order the volumes for a VM
- **replace-vm-credentials**: replace the credentials made available to a VM via
                              the Metadata service. This is primarily for
                              debugging
- **replace-vm-identity**: replace the identity used for generating credentials
                           made available to a VM via the Metadata service
- **replace-vm-image**: replace the root image for a VM. The old root image is
                        saved. The VM must not be running
- **replace-vm-user-data**: replace the user data for a VM. The old user data is
                        saved
- **restore-vm**: restore all VM data (volumes) and metadata from a storage
                  source. If the target *Hypervisor* has the original IP
                  available it will be re-allocated for the new (restored) VM,
                  otherwise a new IP address will be allocated
- **restore-vm-from-snapshot**: restore VM volumes from the previous snapshot,
                                discarding current volumes
- **restore-vm-image**: restore the previously saved root image for a VM. The VM
                        must not be running
- **restore-vm-user-data**: restore the previously saved user data for a VM
- **save-vm**: save (backup) all VM data (volumes) and metadata to a storage
               destination
- **scan-vm-root**: scan the root file-system of stopped VM and write to
                    scanFilename
- **set-vm-migrating**: change the VM state to migrating. For debugging only
- **snapshot-vm**: create a snapshot of the VM volumes, discarding previous one
- **start-vm**: start a stopped VM
- **stop-vm**: stop a running VM. All data and metadata are preserved
- **trace-vm-metadata**: trace the requests a VM makes to the metadata service
- **unset-vm-migrating**: change the VM state to stopped. For debugging only

## Security
The *Hypervisor* restricts RPC access using TLS client authentication.
*vm-control* will load certificate and key files from the
`~/.ssl` directory. *vm-control* will present these certificates to
the *Hypervisor*. If one of the certificates is signed by a certificate
authority that the *Hypervisor* trusts, the *Hypervisor* will grant access.
Most operations only require a certificate that proves *identity*.
The *[Keymaster](https://github.com/Cloud-Foundations/keymaster)* is a good
choice for issuing these certificates.

## Importing virsh (libvirt) VMs
A libvirt VM may be imported into the *Hypervisor*. Once the VM is *committed*
it is removed from the libvirt database and is fully "owned" by the
*Hypervisor*. Importing a VM requires root access on the *Hypervisor* (the
*vm-control* tool will use the `sudo` command if needed).

There are a few simple steps that should be followed to import a VM. In the
example below, the MAC address of the VM to be imported is `52:54:de:ad:be:ef`
and the hostname (DNS entry) is `jump.prod.company.com`. The IP address of the
VM may also be used. In either case, the hostname or IP address provided must
match the libvirt *domain name*. If the VM has multiple network interfaces, the
MAC and IP address/FQDN for each interface must be provided in pairs.
- log into the VM and determine its MAC address
- run `vm-control import-virsh-vm 52:54:de:ad:be:ef jump.prod.company.com`
- enter `shutdown` at the prompt
- wait for the VM to start and log in and check that it is functioning properly
- respond to the `commit/defer/abandon` prompt:
  - `commit`: the VM is removed from the libvirt database
  - `defer`: the VM is left running on the *Hypervisor*. It may later be
             committed (and the `virsh undefine` command should be used to
             remove it from the libvirt database) or destroyed. This is not
             recommended
  - `abandon`: the VM is deleted from the *Hypervisor* and will need to be
               manually started with the `virsh` command

## Exporting VMs to virsh (libvirt)
A local VM on the *Hypervisor* may be exported to a libvirt VM. Once the libvirt
VM is *committed* the original VM is removed from the database and is fully
"owned" by libvirt. Exporting a VM requires root access on the *Hypervisor* (the
*vm-control* tool will use the `sudo` command if needed).

There are a few simple steps that should be followed to export a VM. In the
example below, the hostname (DNS entry) of the VM to be exported is
`jump.prod.company.com`. The IP address of the VM may also be used. In either
case, the hostname or IP address provided will become the new libvirt *domain
name*.
- set up a DHCP server
- add the IP and MAC addresses of the VM being exported to the DHCP server
  configuration. Alternatively, you can log into the VM prior to exporting it
  and reconfigure it to use a static network configuration
- if a *[Fleet Manager](../fleet-manager/README.md)* is being used, the IP address must be added to the
  `ReservedIPs` list in the topology, so as to prevent re-use of the IP address
  when creating new VMs
- run `vm-control export-virsh-vm jump.prod.company.com`
- you will be prompted if a DHCP server entry has been configured
- wait for the VM to start and log in and check that it is functioning properly
- respond to the `commit/defer/abandon` prompt:
  - `commit`: the VM is removed from the *Hypervisor* database
  - `defer`: the new libvirt VM is left running. It may later be committed (and
             the `vm-control destroy-vm` command should be used to
             remove it from the *Hypervisor* database). Deferring is not
             recommended
  - `abandon`: the new libvirt VM is deleted from the libvirt database and the
               original VM will be started

## VM Placement Command
An optional local command to be used when making VM placement decisions (when
creating, copying, migrating or restoring VMs) may be specified using the
`-placementCommand` option when `-placement=command`.
The *placement command* may read from the standard input a JSON payload
describing the `Hypervisors` available (based on other location and tag matching
options) and the `VmInfo` describing the VM that will be placed. The name of
the selected *Hypervisor* must be written to the standard output.

The schema of the data presented to the standard input is:
```
struct {
    Hypervisors []fm_proto.Hypervisor `json:",omitempty"`
    VmInfo      hyper_proto.VmInfo
}
```

See [fm_proto](../../proto/fleetmanager/messages.go) and [hyper_proto](../../proto/hypervisor/messages.go) for sub-schema definitions.

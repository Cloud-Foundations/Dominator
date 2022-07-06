# subd
The daemon that runs on every dominated system.

This daemon continuously checksum scans the root file-system and responds to
**poll**, **fetch files** and **update** RPC requests from the
*[dominator](../dominator/README.md)*.
In order to have a neglibible impact on system workload, it lowers its priority
(nice 15 by default), restricts itself to one CPU and automatically rate limits
its I/O to be 2% of the media speed.

## Status page
*Subd* provides a web interface on port `6969` which provides a status page,
access to performance metrics and logs. If *subd* is running on host `myhost`
then the URL of the main status page is `http://myhost:6969/`. An RPC over HTTP
interface is also provided over the same port.

## Startup
*Subd* is started at boot time, usually by one of the provided
[init scripts](../../init.d/). The *subd* process is baby-sat by the init
script; if the process dies the init script will re-start *subd*. It may be
stopped with the command:

```
service subd stop
```

which also kills the baby-sitting init script. It may be started with the
comand:

```
service subd start
```

There are many command-line flags which may change the behaviour of *subd* but
the defaults should be adequate for most deployments. Built-in help is available
with the command:

```
subd -h
```

## Security
RPC access is restricted using TLS client authentication. *Subd* expects a root
certificate in the file `/etc/ssl/CA.pem` which it trusts to sign certificates
which grant access. It also requires a certificate and key which grant it the
ability to **fetch** files from the objectserver. These should be in the files
`/etc/ssl/subd/cert.pem` and `/etc/ssl/subd/key.pem`, respectively.

If any of these files are missing, *subd* will refuse to start. This prevents
accidental deployments without access control.

## Control and debugging
The *[subtool](../subtool/README.md)* utility may be used to manipulate various
operating parameters of a running *subd* and perform RPC requests.

## DisruptionManager
Disruptive updates can be controlled using an optional *Disruption Manager*
which *subd* can run to request, check and cancel requests to perform a
disruptive upgrade (an upgrade where a *HighImpact* trigger is called). This may
be used to request that new work will not be scheduled on the machine and wait
for existing work to complete before performing the upgrade.

The *Disruption Manager* is a simple tool which takes one of the following
arguments:
- **cancel**: cancel a request to disrupt
- **check**: check whether disruptions are permitted
- **request**: request to perform disruption

Regardless of the argument provided, the tool must return one of the following
exit codes:
- **0**: disruption is permitted
- **1**: disruption has been requested but not yet permitted
- **2**: disruption is denied (not currently permitted)

Once a machine enters the `disruption is permitted state`, it must remain in
that state until a `cancel` command is made, or more than one hour has passed
since the last `request` is made.

The *DisruptionManager* may be called frequently (up to every second) by every
machine in the fleet.

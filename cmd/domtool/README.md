# domtool
A utility to control the *[dominator](../dominator/README.md)*.

The *domtool* utility may be used to control a running *dominator*.
*Domtool* may be run on any machine and can be used to manipulate various
operating parameters of a running *dominator* and perform RPC requests. It is
typically run on a desktop or bastion machine.

## Usage
*Domtool* supports several sub-commands. There are many command-line flags which
provide parameters for these sub-commands. The most commonly used parameter is
`-domHostname` which specifies which host the *dominator* to control is running
on.
The basic usage pattern is:

```
domtool [flags...] command [args...]
```

Built-in help is available with the command:

```
domtool -h
```

Some of the sub-commands available are:

- **clear-safety-shutoff** *sub*: do a one-time clearing of the `unsafe update`
                                  condition for the specified *sub*, allowing
				  the update to continue
- **configure-subs**: set the current configuration of all *subs* (such as rate
                      limits for scanning the file-system and **fetching**
                      objects)
- **disable-updates** *reason*: tell *dominator* to not perform automatic
                                updates of *subs*. The given *reason* must be
                                provided and is logged
- **disruption-cancel** *sub*: cancel disruption for the specified *sub*
- **disruption-check** *sub*: check the disruption state for the specified *sub*
- **disruption-request** *sub*: request disruption for the specified *sub*
- **enable-updates** *reason*: tell *dominator* to perform automatic updates of
                               *subs*. The given *reason* must be provided and
                               is logged
- **fast-update** *sub*: perform a fast update for the specified *sub*
- **force-disruptive-update** *sub*: do a one-time clearing of the `disruption
                                     denied` condition for the specified *sub*,
				     allowing the update to continue
- **get-default-image**: get the default image that will be pushed to and *sub*
                         which does not have a `RequiredImage` specified in the
			 MDB
- **get-info-for-subs**: get information for all/selected *subs* and write to
                         stdout in JSON format
- **get-machine-from-mdb** *sub*: get machine data for the specified *sub* from
                                  the MDB server and write to stdout in JSON
                                  format
- **get-mdb**: get machine data from the MDB server and write to stdout in JSON
               format
- **get-mdb-updates**: get machine data from the MDB server and a stream of
                       updates and write to stdout in JSON format
- **get-subs-configuration**: get the current configuration that is pushed to
                              all *subs*
- **list-subs**: list all/selected *subs* and write to stdout
- **pause-sub-updates** *sub* *reason*: pause updates for the specified *sub*.
                                        The given *reason* must be provided and
					is logged
- **resume-sub-updates** *sub*: resume updates for the specified *sub*
- **set-default-image**: set the default image that will be pushed to and *sub*
                         which does not have a `RequiredImage` specified in the
			 MDB

## Security
*[Dominator](../dominator/README.md)* restricts RPC access using TLS client
authentication. *Domtool* will load certificate and key files from the
`~/.ssl` directory. *Domtool* will present these certificates to *dominator*. If
one of the certificates is signed by a certificate authority that *dominator*
trusts, *dominator* will grant access.

## Critical Sub-Commands
The most important sub-commands are described below for convenience.

### Emergency Stop
To disable automated updates, issue the following command:

```domtool -domHostname=mydom.zone disable-updates "my stop reason"```

This will prevent the *[dominator](../dominator/README.md)* running on the host
`mydom.zone` from performing automated updates. The reason for the emergency
stop along with the username of the person issuing the stop is logged.

### Restart
To enable automated updates, issue the following command:

```domtool -domHostname=mydom.zone enable-updates "my restart reason"```

This will restart automated updates. The reason for the restart (typically an
explanation of why the emergency stop is no longer needed) along with the
username of the person issuing the restart is logged.

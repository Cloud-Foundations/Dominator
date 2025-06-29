# mdbd
The *mdbd* daemon is a companion to the *[dominator](../dominator/README.md)*
daemon. It queries one or more Machine DataBases and provides the *dominator*
with information about machines (*subs*) to manage.

The daemon writes processed and filtered MDB data to a local file
(`/var/lib/mdbd/mdb.json` by default) which the *[dominator](../dominator/README.md)* can consume. Thus,
both *mdbd* and *[dominator](../dominator/README.md)* must run on the same machine.

## Startup
*Mdbd* is started at boot time, usually by one of the provided
[init scripts](../../init.d/). The *mdbd* process is baby-sat by the init
script; if the process dies the init script will re-start it. It may be stopped
with the command:

```
service mdbd stop
```

which also kills the baby-sitting init script. It may be started with the
command:

```
service mdbd start
```

There are many command-line flags which may change the behaviour of *mdbd*
but many have defaults which should be adequate for most deployments. Built-in
help is available with the command:

```
mdbd -h
```

### Key configuration parameters
The init script reads configuration parameters from the `/etc/default/mdbd`
file. The following is the minimum likely set of parameters that will need to be
configured.

The `USERNAME` variable specifies the username that *mdbd* should run as.
Since *mdbd* does not need root privileges, the init script runs
*mdbd* as this user.

### Configuration files
*Mdbd* requires "upstream" data sources. A configuration file
(`/var/lib/mdbd/mdb.sources.list` by default) specifies the data sources
to be collected from.

To directly query AWS for all running instances in a region, you would use the
following configuration file. It will query all the accounts for which you have
API keys in your `~/.aws/credentials` file:

```
aws-local
```

An example configuration file which specifies to collect MDB data from CIS
(Cloud Intelligence Service, being developed at Symantec) for the `us-east-1`
cluster is:

```
cis http://cis.us-east-1.aws.net:9200/aws/aws_nodes/_search?size=10000
```

Since CIS is built on top of Elastic Search, the configuration is primarily an
Elastic Search query.

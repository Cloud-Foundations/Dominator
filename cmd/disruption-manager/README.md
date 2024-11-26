# disruption-manager
A simple Disruption Manager Service.

The *disruption-manager* may be used to limit the number of concurrent disruptive updates to *[subs](../subd/README.md)*. While most updates are usually non-disruptive, some may cause an essential service to restart. These are marked in the image trigger as *HighImpact* so that *[subd](../subd/README.md)* can call the *Disruption Manager*.

This simple Disruption Manager Service reads tags from the MDB data for a machine to determine whether to permit a disruptive update or deny until later. The relevant tags are:
- **DisruptionManagerGroupIdentifier**: an arbitrary group identifier which can be used to separately limit different groups of machines running unrelated services. For example, you might use `NomadNodes` for Nomad workers, `Kubelets` for Kubernetes nodes and `Prometheus` for Prometheus collectors. If unspecified the value of the `RequiredImage` field is used as the group identifier. If the empty string is specified, the machine is counted as part of the default global group. If the group identifier changes while a machine is not in the `denied`
disruption state, the behaviour is undefined
- **DisruptionManagerGroupMaximumDisrupting**: an optional maximum number of concurrent disruptive updates permitted. If unspecified the limit is one
- **DisruptionManagerReadyTimeout**: an optional time to wait after disruption is cancelled for a machine before the next machine can transition to `permitted`. This may be used to give a service instance time to become ready before another instance is disrupted
- **DisruptionManagerReadyUrl**: an optional URL to check after disruption is cancelled for a machine before the next machine can transition to `permitted`. It must return a HTTP 200 status code to signify ready before another service instance is disrupted or until the **DisruptionManagerReadyTimeout** is reached (default 15 minutes if unspecified). Go [template expansion](https://pkg.go.dev/text/template) is applied to this string, using the MDB [Machine](https://pkg.go.dev/github.com/Cloud-Foundations/Dominator/lib/mdb#Machine) data

## Status page
The *disruption-manager* provides a web interface on port `6979` which provides a status page, access to performance metrics and logs. If *disruption-manager* is running on host `myhost` then the URL of the main status page is `http://myhost:6979/`. An RPC over HTTP interface is also provided over the same port.

## Startup
*disruption-manager* is started at boot time, usually by one of the provided [init scripts](../../init.d/). The *disruption-manager* process is baby-sat by the init script; if the process dies the init script will re-start *disruption-manager*. It may be stopped with the command:

```
service disruption-manager stop
```

which also kills the baby-sitting init script. It may be started with the command:

```
service disruption-manager start
```

There are many command-line flags which may change the behaviour of *disruption-manager* but the defaults should be adequate for most deployments. Built-in help is available with the command:

```
disruption-manager -h
```

## Security
RPC access is restricted using TLS client authentication. *Disruption-Manager* expects a root certificate in the file `/etc/ssl/CA.pem` which it trusts to sign certificates which grant access.

## Protocol
The *Disruption Manager* receives requests with MDB data for a machine and the requested operation. The preferred protocol is SRPC. The supported operations are:
- **cancel**: cancel a request to disrupt
- **check**: check whether disruptions are permitted
- **request**: request to perform disruption

Any other request will return an error.

Regardless of the (valid) argument provided, the (new) disruption state is returned, and may be one of the following:
- **denied**: disruption is denied (not currently permitted)
- **permitted**: disruption is permitted
- **requested**: disruption has been requested (and acknowledged) but not yet permitted

A machine which is in **permitted** or **requested** state for more than an hour since the last **request** operation will move to the **denied** state.

### REST Protocol
As an alternative to the SRPC interface, a POST request may be sent to the `/api/v1/request` endpoint, containing a JSON-encoded payload with the machine MDB data and the requested operation. For example, a request for disruption:
```
{
    "MDB": {
        "Hostname": "nomad-node-0",
        "Tags": {
            "BusinessUnit": "core-team",
            "DisruptionManagerGroupIdentifier": "NomadNodes"
        }
    },
    "Request": "request"
}
```
The following response would be returned if disruption is **permitted**:
```
{
    "Response": "permitted"
}
```

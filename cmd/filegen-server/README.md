# filegen-server
The *filegen-server* daemon serves computed files for the **Dominator** system.

The *[dominator](../dominator/README.md)* queries zero or more *filegen-server*
instances when it needs to distribute *computed files*. This *filegen-server* is
a reference implementation and serves some simple computed files. For more
custom types of computed files, see the documentation for the
[lib/filegen](https://godoc.org/github.com/Cloud-Foundations/Dominator/lib/filegen)
package. This reference implementation may be used as a template for writing
your own file generator.

Note that the *[dominator](../dominator/README.md)* will only push *computed
files* to machines if the image being pushed to those machines contains one or
more *computed files*. The address of the *filegen-server* is specified in the
image along with the corresponding pathname in the image. The same image could
reference multiple *computed files*, each being served by a different
*filegen-server* if desired. A common pattern would be to have one
*filegen-server* to serve up non-secret configuration files, and another
*filegen-server* (strongly secured) to serve up secrets.

## Status page
The *filegen-server* provides a web interface on port `6972` which provides a
status page, links to built-in dashboards and access to performance metrics and
logs. If *filegen-server* is running on host `myhost` then the URL of the main
status page is `http://myhost:6972/`. An RPC over HTTP interface is also
provided over the same port.


## Startup
*Filegen-Server* is started at boot time, usually by one of the provided
[init scripts](../../init.d/). The *filegen-server* process is baby-sat by the
init script; if the process dies the init script will re-start it. It may be
stopped with the command:

```
service filegen-server stop
```

which also kills the baby-sitting init script. It may be started with the
command:

```
service filegen-server start
```

There are many command-line flags which may change the behaviour of
*filegen-server* but many have defaults which should be adequate for most
deployments. Built-in help is available with the command:

```
filegen-server -h
```

### Key configuration parameters
The init script reads configuration parameters from the
`/etc/default/filegen-server` file. The following is the minimum likely set of
parameters that will need to be configured.

The `CONFIG_FILE` variable specifies the name of the file from which to read the
configuration.

The `USERNAME` variable specifies the username that *filegen-server* should run
as. Since *filegen-server* does not need root privileges, the init script runs
*filegen-server* as this user.

## Security
RPC access is restricted using TLS client authentication. *Filegen-Server*
expects a root certificate in the file `/etc/ssl/CA.pem` which it trusts to sign
certificates which grant access. It also requires a certificate and key which
clients will use to validate the server. These should be in the files
`/etc/ssl/filegen-server/cert.pem` and `/etc/ssl/filegen-server/key.pem`,
respectively.

## Configuration file
The configuration file contains zero or more lines of the form:
`GeneratorType pathname [args...]`. The generator type specifies an algorithm to
use to generate data for the specified *pathname*. The following generator types
are supported:

- **DynamicTemplateFile** pathname *filename*: the contents of *filename* are
  used as a template to generate the file data. If the file contains sections of
  the form `{{.MyVar}}` then the value of the `MyVar` variable from the MDB for
  the host are used to replace the section. If *filename* changes (replaced with
  a different inode), then the data are regenerated and distributed to all
  machines

- **File** pathname *filename*: the contents of *filename* are used to provide
  the file data. If *filename* changes (replaced with a different inode), then
  the data are regenerated and distributed to all machines

- **MDB** pathname: the file data are the JSON encoding of the MDB data for the
  host

- **MdbFieldDirectory** pathname *field* *directory* [*interval*]: the named
  *field* of the MDB data for the host is used as the filename under the
  specified *directory*. This file contains the data for the host. For example,
  the `Hostname` field would be used to specify a file for every host. If the
  file does not exist the `*` file is read to load default data. An optional
  reload *interval* may be specified

- **Programme** pathname *progpath*: a programme specified by *progpath* is run
  to generate the file data for each machine. The *pathname* will be provided as
  the first argument to the programme. Using this generator type for large
  numbers of machines may consume significant resources.
  The MDB data for the machine will be written to the standard input in JSON
  format.
  The data and number of seconds it is valid (0 means indefinitely valid) must
  be written to the standard output in JSON format, stored in the `Data` and
  `SecondsValid` fields.

- **StaticTemplateFile** pathname *filename*: the contents of *filename* are
  used as a template to generate the file data. If the file contains sections of
  the form `{{.MyVar}}` then the value of the `MyVar` variable from the MDB for
  the host are used to replace the section

- **URL** pathname *url*: a HTTP POST request will be sent to the URL specified
  by *url* containing JSON-encoded MDB data in the request body. The *pathname*
  will be provided as a `pathname=` query paramater.
  The data and number of seconds it is valid (0 means indefinitely valid) must
  be written to the response body in JSON format, stored in the `Data` and
  `SecondsValid` fields.

## Examples
Below are some examples show how to use the different generator types. They show
a sample configuration line for each generator type.

### `DynamicTemplateFile`
```
DynamicTemplateFile /etc/issue.net /var/lib/filegen-server/computed-files/issue.net.template
```
Contents of the `issue.net.template` file:
```
This system is Dominated with image {{.RequiredImage}} and kernel v\r \m \n \l
```
This will generate an `/etc/issue.net` file which shows the required image (as
configured in the MDB) and the installed kernel. If the template file is
replaced, new data will be generated for each machine and pushed.

### `File`
```
File /etc/resolv.conf /var/lib/filegen-server/computed-files/resolve.conf
```
This will push the `resolv.conf` file to all machines. If the file changes, the
new file (inode) will be pushed to all machines. A typical application would be
to have a *[dominator](../dominator/README.md)* in each datacentre (region) and
run the *filegen-server* on the same host. The address of the computed file (as
stored in an image) would be `localhost:6972`, which would tell the
*[dominator](../dominator/README.md)* to connect to the local *filegen-server*.
This would allow you to push different `/etc/resolv.conf` files to different
datacentres.

### `MDB`
```
MDB /etc/mdb.json
```
This will push the MDB contents corresponding to each machine. If the MDB
content for a machine changes, the new content will be pushed. This is a
built-in configuration entry, there is no need to explicitely configure this.

### `MdbFieldDirectory`
```
MdbFieldDirectory /etc/myapp/config Hostname /var/lib/filegen-server/computed-files/myapp 90s
```
In the `/var/lib/filegen-server/computed-files/myapp` directory, you would place
configuration files for each machine. The leaf filename should be the same as
the `Hostname` field in the MDB data. Every 90 seconds the files will be
reloaded and re-pushed if they have changed.

You can also use a tag value as the indexing key to the directory:
```
MdbFieldDirectory /etc/myapp/config Tags.Type /var/lib/filegen-server/computed-files/myapp 90s
```
This will use the value of the `Type` tag as the indexing key.

### `Programme`
```
Programme /etc/ssh/id_rsa     /usr/local/bin/genkey
Programme /etc/ssh/id_rsa.key /usr/local/bin/genkey
```
The `/usr/local/bin/genkey` programme will be called to generate the SSH key
pair, once for the private key and once for the public key/certificate. The
pathname being generated will be passed as the first argument to the programme,
so it can use this to know whether to return the private versus public key.
An example programme might check the `Hostname` field from the MDB data to
decide whether to generate a key (this would essentially be DNS-based
authentication).

The programme must return the data via the standard output in JSON encoding. For
example:
```
{
    "Data": "ZnJlZAo=",
    "SecondsValid": 10
}
```
The encoded data is `fred` (with a trailing newline) and the data are valid for
10 seconds. Note the use of base64 encoding for the `Data` field.

### `StaticTemplateFile`
```
StaticTemplateFile /etc/issue.net /var/lib/filegen-server/computed-files/issue.net.template
```
Contents of the `issue.net.template` file:
```
This system is Dominated with image {{.RequiredImage}} and kernel v\r \m \n \l
```
This will generate an `/etc/issue.net` file which shows the required image (as
configured in the MDB) and the installed kernel.

### `URL`
```
URL /etc/myapp/server-list http://myapp.internal:1234/listServers
```
A HTTP POST query will be sent to the specified URL for each machine, and the
data returned from the query will be pushed to `/etc/myapp/server-list`.
The data must be returned in JSON encoding. See the `Programme` generator for an
example.

## Template file format
The file `lib/filegen/template.go` loads template files and watches for updates.
These template files use the go [text/template](https://pkg.go.dev/text/template@go1.24.5)
standard package. Template functions can be defined and added to the funcMap to process
MDB fields in the template. The following template functions are available:

* `GetSplitPart`: splits a string based on the given separator, then returns a
                substring given the index of the split array
* `ToLower`: returns the lowercase version of a string
* `ToUpper`: returns the uppercase version of a string

More info on template functions: https://pkg.go.dev/text/template@go1.24.5#Template.Funcs

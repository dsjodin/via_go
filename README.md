Custom deployment tool for VMware ESXi Hypervisor
=========================================

Credits
-------

Massive credits go to one of my best friends, and mentor [Jonathan "stamp" G](https://www.github.com/stamp) for all the help, coaching and lessons during this project.
Without your support this project would never have been a reality.

VMware #clarity-ui channel for being super helpful with newbie questions about clarity!


What is go-via?
---------------
go-via is a single binary, that when executed performs the tasks of dhcpd, tftpd, httpd, and ks.cfg generator, with a angular front-end, and http-rest backend written in go, and sqlite for persisting.

Why a new version of VMware Imaging Appliance?
----------------------------------------------
The old version of VIA had some things it didn't support which made it hard to run in enterprise environments. go-via brings added support for the following.
1. IP-Helper , you can have the go-via binary running on any network you want and use [RFC 3046 IP-Helper](https://tools.ietf.org/html/rfc3046) to relay DHCP requests to the server.
2. UEFI , go-via does not support BIOS, but does support UEFI and secure-boot. BIOS may be added in the future.
3. Virtual environments, it does not block nested esxi host deployment.
4. HTTP-REST, everything you can do in the UI, you can do via automation also.
5. Options to perform all prerequisites for VMware Cloud Foundation 4.x/5.x

Supported Architectures
-----------------------
UEFI x86_64 INTEL/AMD architecture
UEFI arm_64 ARM architecture (including Project Monterey/SmartNICs)

Default username / password / port
----------------------
username: admin <br>
password: VMware1!<br>
port: 8443<br>

The admin account is seeded with that password and flagged as needing a change.
go-via warns about it on every start and the UI asks you to replace it at first
login. Do so — this appliance stores and hands out ESXi root passwords.

Authentication
--------------
The UI logs in at `POST /v1/login` and receives an HttpOnly session cookie;
`POST /v1/logout` invalidates it server side. Automation can keep using HTTP
basic auth on any endpoint, which is what the scripts in `example-scripts/` do:

``` bash
curl --user admin:VMware1! --insecure https://localhost:8443/v1/hosts
```

Repeated failures are throttled per account and source address, on both the
login endpoint and basic auth.

Installation / Running
----------------------
<h3> Option 1: docker compose </h3>

This project does not publish container images, so compose builds one locally
from the repository. Nothing is pulled from a registry.

``` bash
git clone https://github.com/dsjodin/via_go.git
cd via_go
sudo docker compose up -d --build
```

Host networking is required rather than merely convenient: the DHCP server has
to see broadcast traffic on the real interfaces, and the boot services bind
67/udp, 69/udp and 80.

State lives in bind mounts next to the compose file:

| directory | contents |
|---|---|
| `database/` | the SQLite database — the host inventory |
| `secret/` | the AES key. **Lose this and every stored ESXi password becomes undecryptable.** |
| `cert/` | the generated CA and server certificate |
| `images/` | uploaded and extracted ESXi images, several GB per release |
| `config/` | optional `config.json` |

By default go-via serves DHCP on every interface it can find. To limit it, put a
config file in `./config` and uncomment the `command:` line in
`docker-compose.yml`.

``` json
{
    "network": {
        "interfaces": ["ens224", "ens192"]
    },
    "port": 443
}
```

<h3> Option 2: Download the latest release, and run ./go-via -file config.json </h3>

Most linux distributions should work, this has been tested on Ubuntu 20.20.

``` bash
#wget the release you want to download, e.g go-via_.<release>_linux_amd64.tar.gz
wget https://github.com/dsjodin/via_go/releases/download/<release>/go-via_.<release>_linux_amd64.tar.gz


#untar/extract it
tar -zxvf go-via_.<release>_linux_amd64.tar.gz
```
This will extract the files README.MD (this document) and go-via binary.

Optional: example config files.

Multi interface, and custom port.
``` json
{
    "network": {
        "interfaces": ["ens224", "ens192"]
    },
    "port": 443
}
```
Single interface, default port 8443
``` json
{
    "network": {
        "interfaces": ["ens224"]
    }
}
```

Now start the binary as super user, (optionally: pointing to the config file.)
``` bash
#start the application with default settings
sudo ./go-via

#start the application with normal debug level
sudo ./go-via -file config.json

#start the application with verbose debug level
sudo ./go-via -file config.json -debug
```

Example systemd.service config file
```
[Unit]
Description=go-via
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=1
User=root
ExecStart=/home/govia/go-via
WorkingDirectory=/home/govia/go-via

[Install]
WantedBy=multi-user.target
```

You should be greeted with the following output.
``` bash
INFO[0000] Startup                                       commit=none date=unknown version=dev
WARN[0000] no interfaces have been configured, trying to find interfaces to serve to, will serve on all. 
INFO[0000] Existing database sqlite-database.db found   
INFO[0000] Starting dhcp server                          int=ens224 ip=172.16.100.1 mac="00:0c:29:91:cf:eb"
INFO[0000] Starting dhcp server                          int=ens192 ip=192.168.1.173 mac="00:0c:29:91:cf:e1"
INFO[0000] Starting dhcp server                          int=docker0 ip=172.17.0.1 mac="02:42:09:9f:04:4f"
INFO[0000] cert                                          server.crt="server.crt found"
INFO[0000] Webserver                                     port=":8443"
```

<h3> Option 3: Build from source </h3>

You need Go (the version is pinned in go.mod) and Node 22+ for the frontend.

``` bash
git clone https://github.com/dsjodin/via_go.git
cd via_go

# build the frontend and regenerate the API docs, then compile
go generate ./...
go build ./cmd/go-via

sudo ./go-via -file config.json
```

`go generate` builds the Next.js app in `ui/` and copies its static export into
`webui/dist`, which is embedded into the binary. A placeholder page is committed
so `go build` works on its own if you have no Node toolchain — the API and the
DHCP, TFTP and boot services are unaffected, only the web UI is missing.

To work on the frontend with hot reloading, run the backend and the Next.js dev
server side by side:

``` bash
# terminal 1
sudo ./go-via -file config.json

# terminal 2
cd ui
npm install
npm run dev
```

Troubleshooting
---------------
To troubleshoot, enable debugging.

Append -debug to the command.
``` bash
sudo ./go-via -debug
or
sudo ./go-via -file config.json -debug
```

Known issues
------------
Please note that go-via is still under heavy development, and there may be bugs. Following is the list of known issues.

currently tracking no known issues! :D

Todo
-----
- Currently no requests have been made for features. Please submit any ideas you have.

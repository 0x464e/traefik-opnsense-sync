<div align='center'>
    <img width="150" src="assets/traefik-opnsense-sync-logo.png"  alt='icon'/>
    <h1>traefik-opnsense-sync</h1>
    <h3>Automated synchronization of DNS overrides from Traefik reverse proxy entries</h3>
</div>

<div align='center'>

[![Go Report Card][go-report-card-shield]][go-report-card-url]
[![GitHub Release][github-release-shield]][github-release-url]
[![discord][discord-shield]][discord-url]

</div>

---

## About The Project

Imagine your homelab is nicely automated, you can automatically deploy new applications using IaC/GitOps practises.
Your local reverse proxy entires are automatically created using Traefik if you deploy a new application,
BUT you still have to manually create DNS overrides to point a given url to your reverse proxy.  
This project removes the last manual step of creating DNS overrides for reverse proxy entries.

The use-case is quite niche, which is why this project was initially supposed to be a short and quick bash script for my
personal use.
However, I decided that I wanted to learn Go and wanted to make a proper Go application out of it with automated
pipelines.

Currently, the application supports only OPNsense Unbound DNS, because that's what I use in my homelab,
as per OPNsense documentation's recommendation. However, if this project were to somehow gain interest,
I could add support for other OPNsense DNS providers and perhaps even other non-OPNsense DNS providers (such as AdGuard
Home or Pi-Hole).

## Features

- Sync once or run as a sync service that polls Traefik API at configurable intervals
- Run as a native binary or Docker container (as a simple image or via Docker Compose)
- Supports dry-runs (no changes made to OPNsense)
- Supports Traefik v3.x (maybe v2.x works? No idea)
- Supports OPNsense Unbound (tested by me actively on v25.x and any future versions, don't know about older versions)

#### Traefik

- Fetches all HTTP routers from Traefik API
- Parses out domains from router rules
    - Supports expanding out regex rules (e.g. ``HostRegexp(`(ha|haos|home-?assistant)\.example\.com`)``)
    - Supports logical operators in rules (e.g.
      ``(Host(`app.example.com`) || Host(`app2.example.com`)) && !Host(`app3.example.com`) && PathPrefix(`/prefix`)``)
- Supports filtering out routers based on entrypoints, providers, and router names
- Supports basic auth for secured Traefik APIs

#### OPNsense

- Manages Unbound DNS override aliases via OPNsense API
- Creates new DNS override aliases for domains found in Traefik routers
- Removes existing DNS override aliases that no longer have a corresponding Traefik router
- Supports API key + secret for secure OPNsense API access
- Doesn't touch other manually/externally created DNS overrides

## Working Principle & Long Explanation

Expand the spoiler below for a long explanation of this app's purpose and how it works.
<details>
<summary>Click to expand</summary>

### Example scenario

You deploy a new local application and want to access the application via `app.mydomain.com`.
So you have a traefik reverse proxy entry (HTTP router & host rule) for `app.mydomain.com` that points to your
application (e.g. `192.168.10.6:1234`).

Great, now you go to `app.mydomain.com` in your browser, but you get 404 (or whatever).
Oh right, it's because your local DNS doesn't know to resolve `app.mydomain.com` to your Traefik reverse proxy.  
Fine, you create a DNS override in your DNS (OPNsense Unbound in this case)  to point `app.mydomain.com` to your Traefik
reverse proxy. Annoying manual step, but whatever.

But now imagine you have 20 applications, maybe you keep adding/removing applications frequently, maybe you have
multiple redirecting aliases for a single application (e.g. both `app.mydomain.com` and `application.mydomain.com` point
to the same application).  
You can imagine how tedious it would become to manually manage all those DNS overrides every time you add/remove an
application or want to add a new alias.  
With this application, you can automatically sync the DNS overrides in OPNsense Unbound based on the Traefik reverse
proxy entries.

### How it works

The Traefik API returns a list of HTTP routers with their properties.
The routers include the host rules that specify the domains they handle.  
Rule example: (unrealistically complex rule, but demonstrates the capabilities)

```
(Host(`app.mydomain.com`) || HostRegexp(`(ha|haos|home-?assistant).mydomain.com`)) && !Host(`app2.mydomain.com`) && PathPrefix(`/prefix`)
```

We can parse out domains that need DNS overrides entries:

- `app.mydomain.com`
- `ha.mydomain.com`
- `haos.mydomain.com`
- `homeassistant.mydomain.com`
- `home-assistant.mydomain.com`

So for each domain above, we need a DNS override entry.  
We could create each of them as DNS host overrides (app.mydomain.com → Traefik IP), but this would clutter the Host
overrides list.  
To solve this problem, OPNsense Unbound supports aliases, where you can map a domain to a specific host override
entry.  
So we create a single host override entry, e.g. `reverse-proxy.mydomain.com` → Traefik IP  
And then we have multiple aliases, e.g. `app.mydomain.com` → `reverse-proxy.mydomain.com`

</details>

## Prerequisites

There are a few things you need to set up once before you can take this application into use

#### OPNsense API Access

You need to create an API key + secret with correct permissions to manage Unbound DNS overrides.  
Quick steps:

1. In OPNsense web UI, navigate to `System` → `Access` → `Users`
2. Create a new user
    - Username e.g. `traefik-sync-api-user`
    - Set scrambled password (i.e. this user cannot be logged in to via password)
    - Set privileges:
        - `Services: Unbound (MVC)`
        - `Services: Unbound DNS: Edit Host and Domain Override`
3. Create & download API key + secret for the user by clicking on the little icon to the right of the user entry in the
   users list

You can now set the API key + secret to the application configuration.

#### Create Initial Reverse Proxy Host Override

As explained in the [Working Principle & Long Explanation](#working-principle--long-explanation) section,
you need to create one host overrides, to which all the automatically managed override alises will point to.
Quick steps:

1. In OPNsense web UI, navigate to `Services` → `Unbound DNS` → `Overrides`
2. Under Hosts, click on the <kbd>+</kbd> button to create a new Host Override
    - Host: e.g. `reverse-proxy`
    - Domain: your local domain, e.g. `mydomain.com`
    - IP Address: IP address of your Traefik reverse proxy
3. Apply changes by pressing the `Apply` button at the bottom of the page

You should now have a DNS override `reverse-proxy.mydomain.com` → Traefik IP.  
Set `reverse-proxy.mydomain.com` to the config entry `opnsense.host_override`.

#### Traefik API Access

If you have
enabled [insecure access to the Traefik API](https://doc.traefik.io/traefik/reference/install-configuration/api-dashboard/#opt-api-insecure),
you should be able to (by default) access the API via directly via Traefik's IP and port 8080:
`http://<traefik-ip>:8080/api/http/routers`.

Exposing insecure access is not recommended though, common alternative ways include creating an HTTP router for the
dasboard/api.  
So you may have a domain such as `traefik.mydomain.com` that points to your Traefik dashboard/api.
But then you of course also need a DNS override for `traefik.mydomain.com` to point to your Traefik reverse proxy.

This is kind of a chicken-and-egg problem since the purpose of this app is to automatically create these DNS
overrides.  
However, just create this one singular DNS override manually in OPNsense Unbound, everything else can be automatically
synced.  
Quick steps:

1. In OPNsense web UI, navigate to `Services` → `Unbound DNS` → `Overrides`
2. If you have multiple Host overrides, click on the one for the reverse proxy to display its aliases
3. Under Aliases, click on the <kbd>+</kbd> button to create a new Host Override Alias
    - Host Override: select the one you created previously, e.g. `reverse-proxy.mydomain.com`
    - Host: e.g. `traefik`
    - Domain: your local domain, e.g. `mydomain.com`
4. Apply changes by pressing the `Apply` button at the bottom of the page

You may also have basic auth enabled for the Traefik API/dashboard, you can set your credentials to the config entries
`traefik.username` and `traefik.password`.

---

Correctly configured OPNsense Unbound might look like:
<details>
<summary>Click to expand screenshot</summary>

![OPNsense Unbound Example](https://i.imgur.com/5yYsm7z.png)

</details>

## Installation & Running

You can either download a pre-built binary from the releases page, build from source, or run via Docker.

<further instructions todo, I will create the pipelines and releases first>

## Configuration

todo

## Common Issues

todo  
tls, traefik api access, non-sense dns overrides from traefik base redirect rule



<!-- MARKDOWN LINKS & IMAGES -->

[go-report-card-shield]: https://goreportcard.com/badge/github.com/0x464e/traefik-opnsense-sync

[go-report-card-url]: https://goreportcard.com/report/github.com/0x464e/traefik-opnsense-sync

[discord-shield]: https://img.shields.io/badge/Discord-join-738ad6?logo=discord&logoColor=white

[discord-url]: https://discord.gg/SQCzaVeBTa

[github-release-shield]: https://img.shields.io/github/v/release/0x464e/traefik-opnsense-sync?logo=github&sort=semver

[github-release-url]: https://github.com/0x464e/traefik-opnsense-sync/releases

# mackerel-plugin-nfs [![Build Status](https://travis-ci.org/y-kuno/mackerel-plugin-nfs.svg?branch=master)](https://travis-ci.org/y-kuno/mackerel-plugin-nfs)

NFS client plugin for mackerel.io agent.  
This repository releases an artifact to Github Releases, which satisfy the format for mkr plugin installer.

## Install

```shell
mkr plugin install y-kuno/mackerel-plugin-nfs
```

## Synopsis

```
mackerel-plugin-nfs [-metric-key-prefix=<prefix>]
```

## Example of mackerel-agent.conf

```text
[plugin.metrics.nfs]
command = "/path/to/mackerel-plugin-nfs"
```
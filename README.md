# CF Restage All
[![Go Lang Version](https://img.shields.io/badge/go-1.15-00ADD8.svg?style=flat)](http://golang.com) 
[![License](https://img.shields.io/github/license/generalmotors/cf-restage-all)](LICENSE)

CF plugin for restaging all applications within a pcf space with minimal downtime.

This plugin was built to allow for mass droplet refreshes within a CF space. This plugin will search for all applications that meet search criteria and create a new droplet, map it to the application then restart. 

## Installation

This plugin requires the CF CLI. You can get the latest version of this plugin by visiting [CF Community Plugins](https://plugins.cloudfoundry.org).
 
To install the plugin, use the following command 

```
$ cf install-plugin -r CF-Community “cf-restage-a”
```

## Usage

```
$ cf restage-all [--a #] [--s started|stopped] [--rt #] [--st #]
```

**--a**&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Restage all applications that contain a droplet older than X days. Default is 0.

**--s**&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Restage all applications in this state. [started|stopped]. Default is started.

**--rt**&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Sets the app restart timeout (seconds) Default is 120.

**--st**&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Sets the build restage timeout (seconds) Default is 120.


## Contributing 

Check out the [contributing](CONTRIBUTING.md) readme for information on how to contribute to the project. 

## License 

This project is released under the MIT software license. More information can be found in the [LICENSE](LICENSE) file.
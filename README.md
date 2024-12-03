<a name="readme-top"></a>

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]

<!-- PROJECT LOGO -->
<div align="center">
  <!-- <a href="https://github.com/jorgerojas26/lazysql"> -->
  <!--   <img src="images/logo.png" alt="Logo" width="80" height="80"> -->
  <!-- </a> -->

  <h3 align="center">LAZYSQL</h3>

  <p align="center">
        A cross-platform TUI database management tool written in Go.
  </p>
</div>

<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li><a href="#features">Features</a></li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#installation">Installation</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#commands">Commands</a></li>
    <li><a href="#keybindings">Keybindings</a></li>
    <li><a href="#roadmap">Roadmap</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>

<!-- ABOUT THE PROJECT -->

## About The Project

![Product Name Screen Shot][product-screenshot1]
![Product Name Screen Shot][product-screenshot2]

This project is heavily inspired by [Lazygit](https://github.com/jesseduffield/lazygit), which I think is the best TUI client for Git.

I wanted to have a tool like that, but for SQL. I didn't find one that fits my needs, so I created one myself.

I live in the terminal, so if you are like me, this tool can become handy for you too.

This is my first Open Source project, also, this is my first Go project. I am not a brilliant programmer. I am just a typical JavaScript developer that wanted to learn a new language, I also wanted a TUI SQL Client, so white and bottled.

This project is in ALPHA stage, please feel free to complain about my spaghetti code.

I use Lazysql daily in my full-time job as a full-stack javascript developer in its current (buggy xD) state. So, the plan is to improve and fix my little boy as a side-project in my free time.

### Built With

![Golang][golang-shield]
![Golang][tview-shield]

## Features

- [x] Cross-platform (macOS, Windows, Linux)
- [x] Vim Keybindings
- [x] Can manage multiple connections (Backspace)
- [x] Tabs
- [x] SQL Editor (CTRL + e)

<!-- GETTING STARTED -->

## Getting Started

### Installation

#### Homebrew

```bash
brew tap jorgerojas26/lazysql
brew install lazysql
```

#### Install with go package manager

```bash
go install github.com/jorgerojas26/lazysql@latest
```

#### Binary Releases

For Windows, macOS or Linux, you can download a binary release [here](https://github.com/jorgerojas26/lazysql/releases)

#### Third party (maintained by the community)

Arch Linux users can install it from the AUR with:

```bash
paru -S lazysql

```

or

```bash
yay -S lazysql

```

or install it manual with:

```bash
git clone https://aur.archlinux.org/lazysql.git
cd lazysql
makepkg -si
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- USAGE EXAMPLES -->

## Usage

```bash
$ lazysql
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Support

- [x] MySQL
- [x] PostgreSQL
- [x] SQLite
- [ ] MSSQL
- [ ] MongoDB

Support for multiple RDBMS is a work in progress.

<!-- COMMANDS -->

## Commands

In some cases, mostly when connecting to remote databases, it might be necessary to run a custom command
before being able to connect to the database. For example when you can only access the database through
a remote bastion, you would probably first need to open an SSH tunnel by running the following command
in a separate terminal:

```bash
ssh remote-bastion -L 5432:localhost:5432
```

In order to make it easier to run these commands, lazysql supports running custom commands before connecting
to the database. You can define these commands in the configuration file like this:

```toml
[[database]]
Name = 'server'
Provider = 'postgres'
DBName = 'foo'
URL = 'postgres://postgres:password@localhost:${port}/foo'
Commands = [
  { Command = 'ssh -tt remote-bastion -L ${port}:localhost:5432', WaitForPort = '${port}' }
]
```

The `Command` field is required and can contain any command that you would normally run in your terminal.
The `WaitForPort` field is optional and can be used to wait for a specific port to be open before continuing.

When you define the `${port}` variable in the URL field, lazysql will automatically replace it with a random
free port number. This port number will then be used in the connection URL and is available in the `Commands`
field so that you can use it to configure the command.

You can even chain commands to, for example, connect to a remote server and then to a postgres container
running in a remote k8s cluster:

```toml
[[database]]
Name = 'container'
Provider = 'postgres'
DBName = 'foo'
URL = 'postgres://postgres:password@localhost:${port}/foo'
Commands = [
  { Command = 'ssh -tt remote-bastion -L 6443:localhost:6443', WaitForPort = '6443' },
  { Command = 'kubectl port-forward service/postgres ${port}:5432 --kubeconfig /path/to/kube.conf', WaitForPort = '${port}' }
]
```

<!-- KEYBINDINGS -->

## Keybindings

### Global

| Key       | Action                         |
| --------- | ------------------------------ |
| q         | Quit                           |
| CTRL + e  | Open SQL editor                |
| Backspace | Return to connection selection |
| ?         | Show keybindings popup         |

### Table

| Key      | Action                               |
| -------- | ------------------------------------ |
| c        | Edit table cell                      |
| d        | Delete row                           |
| o        | Add row                              |
| /        | Focus the filter input or SQL editor |
| CTRL + s | Commit changes                       |
| >        | Next page                            |
| <        | Previous page                        |
| K        | Sort ASC                             |
| J        | Sort DESC                            |
| H        | Focus tree panel                     |
| [        | Focus previous tab                   |
| ]        | Focus next tab                       |
| X        | Close current tab                    |
| R        | Refresh the current table            |

### Tree

| Key | Action                         |
| --- | ------------------------------ |
| L   | Focus table panel              |
| G   | Focus last database tree node  |
| g   | Focus first database tree node |

### SQL Editor

| Key          | Action                            |
| ------------ | --------------------------------- |
| CTRL + R     | Run the SQL statement             |
| CTRL + Space | Open external editor (Linux only) |

Specific editor for lazysql can be set by `$SQL_EDITOR`.

Specific terminal for opening editor can be set by `$SQL_TERMINAL`

## Example connection URLs

```
postgres://user:pass@localhost/dbname
pg://user:pass@localhost/dbname?sslmode=disable
mysql://user:pass@localhost/dbname
mysql:/var/run/mysqld/mysqld.sock
sqlserver://user:pass@remote-host.com/dbname
mssql://user:pass@remote-host.com/instance/dbname
ms://user:pass@remote-host.com:port/instance/dbname?keepAlive=10
oracle://user:pass@somehost.com/sid
sap://user:pass@localhost/dbname
file:myfile.sqlite3?loc=auto
/path/to/sqlite/file/test.db
odbc+postgres://user:pass@localhost:port/dbname?option1=
```

<!-- ROADMAP -->

## Roadmap

- [ ] Support for NoSQL databases
- [ ] Columns and indexes creation through TUI
- [x] Table tree input filter
- [ ] Custom keybindings
- [x] Show keybindings on a modal
- [x] Rewrite row `create`, `update` and `delete` logic

See the [open issues](https://github.com/jorgerojas26/lazysql/issues) for a full list of proposed features (and known issues).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Clipboard support

We use [atotto/clipboard](https://github.com/atotto/clipboard?tab=readme-ov-file#clipboard-for-go) to copy to clipboard.

Platforms:

- OSX
- Windows 7 (probably work on other Windows)
- Linux, Unix (requires 'xclip' or 'xsel' command to be installed)

<!-- CONTRIBUTING -->

## Contributing

Contributions, issues, and pull requests are welcome!

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->

## License

Distributed under the MIT License. See `LICENSE.txt` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTACT -->

## Contact

Jorge Rojas - [LinkedIn](https://www.linkedin.com/in/jorgerojas26/) - jorgeluisrojasb@gmail.com

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Alternatives

- [Mitzasql](https://github.com/vladbalmos/mitzasql)
- [Gobang](https://github.com/TaKO8Ki/gobang)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->

[contributors-shield]: https://img.shields.io/github/contributors/jorgerojas26/lazysql?style=for-the-badge
[contributors-url]: https://github.com/jorgerojas26/lazysql/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/jorgerojas26/lazysql?style=for-the-badge
[forks-url]: https://github.com/jorgerojas26/lazysql/network/members
[stars-shield]: https://img.shields.io/github/stars/jorgerojas26/lazysql?style=for-the-badge
[stars-url]: https://github.com/jorgerojas26/lazysql/stargazers
[issues-shield]: https://img.shields.io/github/issues/jorgerojas26/lazysql?style=for-the-badge
[issues-url]: https://github.com/jorgerojas26/lazysql/issues
[license-shield]: https://img.shields.io/github/license/jorgerojas26/lazysql.svg?style=for-the-badge
[license-url]: https://github.com/jorgerojas26/lazysql/blob/main/LICENSE.txt
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/jorgerojas26
[product-screenshot1]: images/lazysql-connection-selection.png
[product-screenshot2]: images/lazysql.png
[golang-shield]: https://img.shields.io/badge/Golang-gray?style=for-the-badge&logo=go
[tview-shield]: https://img.shields.io/badge/tview-gray?style=for-the-badge&logo=go

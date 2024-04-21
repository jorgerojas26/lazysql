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

This project is heavily inspired by [Lazygit](https://github.com/jesseduffield/lazygit), which i think is the best TUI client for Git.

I wanted to have a tool like that, but for SQL. I didn't find one that fits my needs so i created one myself.

I live in the terminal, so if you are like me, this tool can become handy for you too.

This is my first Open Source project, also, this is my first Golang project. I am not a brilliant programmer. I am just a typical Javascript developer that wanted to learn a new language, i also wanted a TUI SQL Client, so, white and bottled.

This project is in ALPHA stage, please feel free to critize my spaghetti code.

I use Lazysql daily in my ful time job as a fullstack javascript developer in it's current (buggy xD) state. So, the plan is to improve and fix my little boy as a side project in my free time.

### Built With

![Golang][golang-shield]
![Golang][tview-shield]

## Features

- [x] Cross-platform (macOS, Windows, Linux)
- [x] VIM Keybindings
- [x] Can manage multiple connections (Backspace)
- [x] Tabs
- [x] SQL Editor (CTRL + e)

<!-- GETTING STARTED -->

## Getting Started

### Installation

#### Binary Releases

For Windows, Mac OS or Linux, you can download a binary release [here](https://github.com/jorgerojas26/lazysql/releases)

#### Install with go package manager

```bash
go install github.com/jorgerojas26/lazysql@latest
```

#### Install from package manager

Archlinux users can install it from the AUR with:

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
- [x] Postgres
- [x] SQLite
- [ ] MSSQL
- [ ] MongoDB

Support for multiple RDBMS is a work in progress.

## Keybindings

### Global

| Key       | Action                         |
| --------- | ------------------------------ |
| ?         | Help                           |
| q         | Quit                           |
| CTRL + e  | Open SQL editor                |
| Backspace | Return to connection selection |

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

### Tree

| Key | Action            |
| --- | ----------------- |
| L   | Focus table panel |

### SQL Editor

| Key      | Action                |
| -------- | --------------------- |
| CTRL + R | Run the SQL statement |

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
sqlite:/path/to/file.db
file:myfile.sqlite3?loc=auto
odbc+postgres://user:pass@localhost:port/dbname?option1=
```

<!-- ROADMAP -->

## Roadmap

- [ ] Support for NOSQL databases
- [ ] Columns and indexes creation through TUI
- [ ] Table tree input filter
- [ ] Custom keybindings
- [ ] Show keybindings on a modal
- [ ] Rewrite row `create`, `update` and `delete` logic

See the [open issues](https://github.com/jorgerojas26/lazysql/issues) for a full list of proposed features (and known issues).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTRIBUTING -->

## Contributing

Contributions, issues and pull requests are welcome!

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- LICENSE -->

## License

Distributed under the MIT License. See `LICENSE.txt` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

<!-- CONTACT -->

## Contact

Jorge Rojas - [Linkedin](https://www.linkedin.com/in/jorgerojas26/) - jorgeluisrojasb@gmail.com

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

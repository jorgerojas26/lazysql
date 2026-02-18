<a name="readme-top"></a>

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url] [![Stargazers][stars-shield]][stars-url]
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
    <li><a href="#environment-variables">Environment variables</a></li>
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

This project is heavily inspired by
[Lazygit](https://github.com/jesseduffield/lazygit), which I think is the best
TUI client for Git.

I wanted to have a tool like that, but for SQL. I didn't find one that fits my
needs, so I created one myself.

I live in the terminal, so if you are like me, this tool can become handy for
you too.

This is my first Open Source project, also, this is my first Go project. I am
not a brilliant programmer. I am just a typical JavaScript developer that wanted
to learn a new language, I also wanted a TUI SQL Client, so blanca y en botella,
leche! (white and bottled).

This project is in ALPHA stage, please feel free to complain about my spaghetti
code.

I use Lazysql daily in my full-time job as a full-stack javascript developer in
its current (buggy xD) state. So, the plan is to improve and fix my little boy
as a side-project in my free time.

### Built With

![Golang][golang-shield] ![Golang][tview-shield]

## Features

- [x] Cross-platform (macOS, Windows, Linux)
- [x] Vim Keybindings
- [x] Can manage multiple connections (Backspace)
- [x] Tabs
- [x] SQL Editor (CTRL + e)

<!-- GETTING STARTED -->

## Getting Started

### Installation

#### Homebrew (macOS/Linux)

```bash
$ brew install lazysql
```

#### Install with go package manager

```bash
go install github.com/jorgerojas26/lazysql@latest
```

#### Binary Releases

For Windows, macOS or Linux, you can download a binary release
[here](https://github.com/jorgerojas26/lazysql/releases)

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

## Configuration

If the `XDG_CONFIG_HOME` environment variable is set, the configuration file
will be located at:

- `${XDG_CONFIG_HOME}/lazysql/config.toml`

If not, the configuration file will be located at:

- Windows: `%APPDATA%\lazysql\config.toml`
- macOS: `~/Library/Application Support/lazysql/config.toml`
- Linux: `~/.config/lazysql/config.toml`

The configuration file is a TOML file and can be used to define multiple
connections.

### Example configuration

```toml
[[database]]
Name = 'Production database'
Provider = 'postgres'
DBName = 'foo'
URL = 'postgres://${user}:urlencodedpassword@localhost:${port}/foo'
ReadOnly = true
Commands = [
  { Command = 'ssh -tt remote-bastion -L ${port}:localhost:5432', WaitForPort = '${port}' },
  { Command = 'whoami', SaveOutputTo = 'user' },
]
[[database]]
Name = 'Development database'
Provider = 'postgres'
DBName = 'foo'
URL = 'postgres://postgres:urlencodedpassword@localhost:5432/foo'
[application]
DefaultPageSize = 300
DisableSidebar = false
SidebarOverlay = false
```

The `ReadOnly` field (optional, defaults to `false`) can be set to `true` to
enable read-only mode for a connection. When enabled, all mutation queries
(INSERT, UPDATE, DELETE, DROP, etc.) will be blocked.

The `[application]` section is used to define some app settings. Not all
settings are available yet, this is a work in progress.

## Usage

> For a list of keyboard shortcuts press `?`

Open the TUI with:

```console
$ lazysql
```

To launch lazysql with the ability to pick from the saved connections.

```console
$ lazysql [connection_url]
```

To launch lazysql and connect to database at [connection_url].

```console
$ lazysql --read-only [connection_url]
```

To launch lazysql in read-only mode.

### Connect to a DB

1. Start `lazysql`
2. Create a new connection (press `n`)
3. Provide a name for the connection as well as the URL to connect to (see
   <a href="#example-connection-urls">example connection URL</a>)
4. Connect to the DB (press `<Enter>`)

If you already have a connection set up:

1. Start `lazysql`
2. Select the right connection (press `j` and `h` for navigation)
3. Connect to the DB (press `c` or `<Enter>`)

### Create a table

There is currently no way to create a table from the TUI. However you can run
the query to create the table as a SQL-Query, inside the
<a href="#execute-sql-queries">SQL Editor</a>.

You can update the tree by pressing `R`, so you can see your newly created
table.

### Execute SQL queries

1. Press `<Ctrl+E>` to open the built-in SQL Editor
2. Write the SQL query
3. Press `<Ctrl+R>` to execute the SQL query

> To switch back to the table-tree press `H`
>
> After executing a `SELECT`-query a table will be displayed under the
> SQL-Editor with the query-result.\
> To switch focus back to SQL-Editor press `/`

### Open/view a table

1. Expand the table-tree by pressing `e` or `<Enter>`
2. Select the table you want to view
   - next node `j`
   - previous node `k`
   - last node `G`
   - first node `g`
3. Press `<Enter>` to open the table

> To switch back to the table-tree press `H`\
> To switch back to the table press `L`

### Filter rows

1. [Open a table](#openview-a-table)
2. Press `/` to focus the filter input
3. Write a `WHERE`-clause to filter the table
4. Press `<Enter>` to submit your filter

> To remove the filter, focus the filter input (press `/`) and press `<Esc>`.

### Insert a row

1. [Open a table](#openview-a-table)
2. Press `1` to switch to the record tab
3. Press `o` to insert a new row
4. Fill out all columns
5. Press `<Ctrl+S>` to save the changes

### Edit a column

1. [Open a table](#openview-a-table)
2. Press `1` to switch to the record tab
3. Move to the column you want to edit
4. Press `c` to edit, Press `<Enter>` to submit
5. Press `<Ctrl+S>` to save the changes

### Export to CSV

#### From Table View

1. [Open a table](#openview-a-table)
2. Apply filters or sorting as needed
3. Press `E` to open the export dialog
4. Optionally modify the file path and batch size
5. Select export scope:
   - Export Current Page: Export only the currently displayed rows
   - Export All Records: Fetch and export all records from the table

> Batch size (default: 10000): When exporting all records, data is fetched in
> batches to avoid timeout or memory issues with large tables. Increase for
> faster exports, decrease if you encounter any errors.
>
> The default file path is `~/Downloads/{database}_{table}_{timestamp}.csv`.

#### From SQL Editor

1. [Execute a SQL query](#execute-sql-queries)
2. Press `E` to open the export dialog
3. Optionally modify the file path
4. Select **Export** to save all query results

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Support

- [x] MySQL
- [x] PostgreSQL
- [x] SQLite
- [x] MSSQL
- [ ] MongoDB

Support for multiple RDBMS is a work in progress.

<!-- COMMANDS -->

## Commands

In some cases, mostly when connecting to remote databases, it might be necessary
to run a custom command before being able to connect to the database. For
example when you can only access the database through a remote bastion, you
would probably first need to open an SSH tunnel by running the following command
in a separate terminal:

```bash
ssh remote-bastion -L 5432:localhost:5432
```

In order to make it easier to run these commands, lazysql supports running
custom commands before connecting to the database. You can define these commands
in the configuration file like this:

```toml
[[database]]
Name = 'server'
Provider = 'postgres'
DBName = 'foo'
URL = 'postgres://${user}:password@localhost:${port}/foo'
Commands = [
  { Command = 'ssh -tt remote-bastion -L ${port}:localhost:5432', WaitForPort = '${port}' },
  { Command = 'whoami', SaveOutputTo = 'user' },
]
```

The `Command` field is required and can contain any command that you would
normally run in your terminal. The `WaitForPort` field is optional and can be
used to wait for a specific port to be open before continuing. The
`SaveOutputTo` field is optional and can be used to make user-defined variables.
The output (`stdout`) from the command will be saved into the variable, and the
variable can be used in the URL or future commands via the `${VARIABLE}` syntax.

When you define the `${port}` variable in the URL field, lazysql will
automatically replace it with a random free port number. This port number will
then be used in the connection URL and is available in the `Commands` field so
that you can use it to configure the command.

You can even chain commands to, for example, connect to a remote server and then
to a postgres container running in a remote k8s cluster:

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

## Environment variables

You can use environment variables in the configuration file using the
`${env:VAR_NAME}` syntax. This is useful for keeping sensitive information like
passwords out of the configuration file.

```toml
[[database]]
Name = 'Production'
Provider = 'postgres'
URL = 'postgres://${env:DB_USER}:${env:DB_PASSWORD}@localhost:5432/mydb'
```

```bash
export DB_USER=admin
export DB_PASSWORD=secret
lazysql
```

Note: Undefined environment variables will be replaced with an empty string.

<!-- KEYBINDINGS -->

## Keybindings

### Custom Keybindings

You can customize keybindings by adding a `[keymap.<Group>]` section to your
`config.toml` file. Each entry maps a command name to a key.

```toml
[keymap.Home]
SwitchToEditorView = "i"
Quit = "Esc"

[keymap.Tree]
GotoTop = "t"
Search = "Ctrl-F"
```

For single character keys, use the character directly (e.g., `"q"`, `"G"`,
`"1"`, `"/"`). For special keys, use the
[tcell key name](https://github.com/gdamore/tcell/blob/v2.7.4/key.go#L83) (e.g.,
`"Enter"`, `"Esc"`, `"Ctrl-S"`). Only key names defined in tcell are supported.

Available groups: `Home`, `Connection`, `Tree`, `TreeFilter`, `Table`, `Editor`,
`Sidebar`, `QueryPreview`, `QueryHistory`, `JSONViewer`.

### Default Keybindings

#### Home

| Default Key | Command                 | Description                |
| ----------- | ----------------------- | -------------------------- |
| L           | MoveRight               | Focus table                |
| H           | MoveLeft                | Focus tree                 |
| Ctrl-E      | SwitchToEditorView      | Open SQL editor            |
| Ctrl-S      | Save                    | Execute pending changes    |
| q           | Quit                    | Quit                       |
| Backspace   | SwitchToConnectionsView | Switch to connections list |
| ?           | HelpPopup               | Help                       |
| Ctrl-P      | SearchGlobal            | Global search              |
| Ctrl-_      | ToggleQueryHistory      | Toggle query history modal |
| T           | ToggleTree              | Toggle file tree           |
| Ctrl-H      | TreeWidthDecrease       | Decrease tree width        |
| Ctrl-L      | TreeWidthIncrease       | Increase tree width        |

#### Connection

| Default Key | Command          | Description                      |
| ----------- | ---------------- | -------------------------------- |
| n           | NewConnection    | Create a new database connection |
| c           | Connect          | Connect to database              |
| Enter       | Connect          | Connect to database              |
| e           | EditConnection   | Edit a database connection       |
| d           | DeleteConnection | Delete a database connection     |
| q           | Quit             | Quit                             |

#### Tree

| Default Key | Command           | Description               |
| ----------- | ----------------- | ------------------------- |
| g           | GotoTop           | Go to top                 |
| G           | GotoBottom        | Go to bottom              |
| Enter       | Execute           | Open                      |
| j           | MoveDown          | Go down                   |
| Down        | MoveDown          | Go down                   |
| Ctrl-U      | PagePrev          | Go page up                |
| Ctrl-D      | PageNext          | Go page down              |
| k           | MoveUp            | Go up                     |
| Up          | MoveUp            | Go up                     |
| /           | Search            | Search                    |
| n           | NextFoundNode     | Go to next found node     |
| N           | PreviousFoundNode | Go to previous found node |
| p           | PreviousFoundNode | Go to previous found node |
| P           | NextFoundNode     | Go to next found node     |
| c           | TreeCollapseAll   | Collapse all              |
| e           | ExpandAll         | Expand all                |
| R           | Refresh           | Refresh tree              |

#### Tree Filter

| Default Key | Command           | Description               |
| ----------- | ----------------- | ------------------------- |
| Esc         | UnfocusTreeFilter | Unfocus tree filter       |
| Enter       | CommitTreeFilter  | Commit tree filter search |

#### Table

| Default Key | Command            | Description                              |
| ----------- | ------------------ | ---------------------------------------- |
| /           | Search             | Search                                   |
| c           | Edit               | Change cell                              |
| d           | Delete             | Delete row                               |
| w           | GotoNext           | Go to next cell                          |
| b           | GotoPrev           | Go to previous cell                      |
| $           | GotoEnd            | Go to last cell                          |
| 0           | GotoStart          | Go to first cell                         |
| y           | Copy               | Copy cell value to clipboard             |
| o           | AppendNewRow       | Append new row                           |
| O           | DuplicateRow       | Duplicate row                            |
| J           | SortDesc           | Sort descending                          |
| R           | Refresh            | Refresh the current table                |
| K           | SortAsc            | Sort ascending                           |
| C           | SetValue           | Toggle value menu (NULL, EMPTY, DEFAULT) |
| [           | TabPrev            | Switch to previous tab                   |
| ]           | TabNext            | Switch to next tab                       |
| {           | TabFirst           | Switch to first tab                      |
| }           | TabLast            | Switch to last tab                       |
| X           | TabClose           | Close tab                                |
| >           | PageNext           | Switch to next page                      |
| <           | PagePrev           | Switch to previous page                  |
| 1           | RecordsMenu        | Switch to records menu                   |
| 2           | ColumnsMenu        | Switch to columns menu                   |
| 3           | ConstraintsMenu    | Switch to constraints menu               |
| 4           | ForeignKeysMenu    | Switch to foreign keys menu              |
| 5           | IndexesMenu        | Switch to indexes menu                   |
| S           | ToggleSidebar      | Toggle sidebar                           |
| s           | FocusSidebar       | Focus sidebar                            |
| Z           | ShowRowJSONViewer  | Toggle JSON viewer for row               |
| z           | ShowCellJSONViewer | Toggle JSON viewer for cell              |
| E           | ExportCSV          | Export to CSV                            |

#### Editor

| Default Key | Command              | Description             |
| ----------- | -------------------- | ----------------------- |
| Ctrl-R      | Execute              | Execute query           |
| Esc         | UnfocusEditor        | Unfocus editor          |
| Ctrl-Space  | OpenInExternalEditor | Open in external editor |

Specific editor for lazysql can be set by `$SQL_EDITOR`.

Specific terminal for opening editor can be set by `$SQL_TERMINAL`

#### Sidebar

| Default Key | Command        | Description                              |
| ----------- | -------------- | ---------------------------------------- |
| s           | UnfocusSidebar | Focus table                              |
| S           | ToggleSidebar  | Toggle sidebar                           |
| j           | MoveDown       | Focus next field                         |
| k           | MoveUp         | Focus previous field                     |
| g           | GotoStart      | Focus first field                        |
| G           | GotoEnd        | Focus last field                         |
| c           | Edit           | Edit field                               |
| Enter       | CommitEdit     | Add edit to pending changes              |
| Esc         | DiscardEdit    | Discard edit                             |
| C           | SetValue       | Toggle value menu (NULL, EMPTY, DEFAULT) |
| y           | Copy           | Copy value to clipboard                  |

#### Query Preview

| Default Key | Command | Description             |
| ----------- | ------- | ----------------------- |
| Ctrl-S      | Save    | Execute queries         |
| q           | Quit    | Quit                    |
| y           | Copy    | Copy query to clipboard |
| d           | Delete  | Delete query            |

#### Query History

| Default Key | Command            | Description                |
| ----------- | ------------------ | -------------------------- |
| s           | Save               | Save query                 |
| d           | Delete             | Delete query               |
| q           | Quit               | Quit                       |
| y           | Copy               | Copy query to clipboard    |
| /           | Search             | Search                     |
| Ctrl-_      | ToggleQueryHistory | Toggle query history modal |
| [           | TabPrev            | Switch to previous tab     |
| ]           | TabNext            | Switch to next tab         |

#### JSON Viewer

| Default Key | Command              | Description             |
| ----------- | -------------------- | ----------------------- |
| Z           | ShowRowJSONViewer    | Toggle JSON viewer      |
| z           | ShowCellJSONViewer   | Toggle JSON viewer      |
| y           | Copy                 | Copy value to clipboard |
| w           | ToggleJSONViewerWrap | Toggle word wrap        |

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
- [x] Custom keybindings
- [x] Show keybindings on a modal
- [x] Rewrite row `create`, `update` and `delete` logic

See the [open issues](https://github.com/jorgerojas26/lazysql/issues) for a full
list of proposed features (and known issues).

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Clipboard support

We use
[atotto/clipboard](https://github.com/atotto/clipboard?tab=readme-ov-file#clipboard-for-go)
to copy to clipboard.

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

Jorge Rojas - [LinkedIn](https://www.linkedin.com/in/jorgerojas26/) -
jorgeluisrojasb@gmail.com

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

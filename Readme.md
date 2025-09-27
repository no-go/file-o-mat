# File-o-mat

Fileomat is a simple system to share files online. Simple `WWW-Authenticate` is used!

![Admin can upload and delete files](Screenshot.png)

## Features webGUI

- Normal User
  - Login
  - browse folders
  - see file size
  - download
- Admin User
  - upload files
  - delete files

## Features backend

- config file to handle...
  - base url and link-prefix
  - name of the log file
  - name of the style- and template file
  - byte limit for upload
  - foldername for file storage
  - logins
  - BAN LIST
    - max number of fails to ban an ip (`max_failed` 8)
    - how many minutes the ip will ban (`block_duration` 30m)
    - the period to check and remove ip from ban list (`check_duration` 5m)
- a simple html template
- inside `core/core.go` 3 template strings to format the file-lines
- sanatize filename: only small letters, numbers and a single . for file extension is allowed
- [Code documentation](#code-documentation)

## ToDo

- examples and unit tests
- run tests via CI/CD
- test with windows
- clean code for handling folder, not allowed paths and file requests (actual a bit chaotic)
- create/delete folders as admin via WebGUI
- display static images
- User management: better password configuration
- remove `main.go` and make an example instead
  - place `core/*.go` to the root
  - make config in that way: place the `log` file and the `upload` file anywhere
- the 3 mini `file` string templates should be part of the "example" or config file

## User management

It is hacky! Run `go run . secretpassword` to get a hash for the `secretpassword`.
Use this hash in `etc/config.json` to set a password or add a user.

See `etc/config.json` which username is the `admin_user`.

### Default

| user      | password     |
|:----------|:-------------|
| tux       | tux          |
| admin     | toor         |

## Install and run

Code tested with debian trixie. Install go 1.24!

```
git clone https://github.com/no-go/file-o-mat.git
cd file-o-mat
go mod tidy
go run .
```

Alternative: You can run `go install github.com/no-go/file-o-mat` but
the `go/pkg/mod/github.com/no-go/file-o-mat/` is not writeable,
if you run `go/bin/file-o-mat` there. Thus log and upload fail!

You should be able to open `http://localhost:60081/fileomat/` in your browser.
Default login see [User management](#user-management).

- To run as "Deamon" the usage of *nohup* is a nice hack: `nohup go run . >>/dev/null 2>>/dev/null &`
- Stop it with: `killall fileomat`

## nginx and location is subfolder

If you want to run behind nginx and inside the subfolder *fileomat* (the default):

```
location /fileomat/ {
	proxy_pass http://localhost:60081/fileomat/;
	client_max_body_size 10M;
	proxy_set_header Host $host;
	proxy_set_header X-Real-IP $remote_addr;
	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
	proxy_set_header X-Forwarded-Proto $scheme;
}
```

Without the `/fileomat/` inside `proxy_pass` you got trouble in link- an folder handling!

## Code documentation

see [pkg.go.dev/github.com/no-go/file-o-mat/core](https://pkg.go.dev/github.com/no-go/file-o-mat/core)

## License

It is under http://unlicense.org
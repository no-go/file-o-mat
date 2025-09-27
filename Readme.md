# Fileomat

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
  - BAN LIST
    - max number of fails to ban an ip (`max_failed` 8)
    - how many minutes the ip will ban (`block_duration` 30m)
    - the period to check and remove ip from ban list (`check_duration` 5m)
- a simple html template
- inside `main.go` 3 template strings to format the file-lines
- sanatize filename: only small letters, numbers and a single . for file extension is allowed

## ToDo

- examples and unit tests
- build and test via CI/CD
- clean code for handling folder, not allowed paths and file requests (actual a bit chaotic)

## Missing (for ever)

- create/delete folders as admin via WebGUI
- display static images
- User management: better password configuration

## User management

It is hacky! Run `go run . secretpassword` to get a hash for the `secretpassword`.
Use this hash in `main.go` to set a password or add a user.

See `etc/config.json` which username is the `admin_user`.

Default user: tux, password tux

Default user: admin, password toor

## Install and run

Code tested with debian trixie. Install go 1.24!

```
git clone https://github.com/no-go/file-o-mat.git
cd file-o-mat
go mod tidy
go run .
```

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

To see some inline comments and how the code works, use `godoc`.
```
go install golang.org/x/pkgsite/cmd/pkgsite@latest
pkgsite
```

Open `http://localhost:8080` in your browser.

## License

It is under http://unlicense.org
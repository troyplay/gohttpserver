# gohttpserver
[![Build Status](https://travis-ci.org/codeskyblue/gohttpserver.svg?branch=master)](https://travis-ci.org/codeskyblue/gohttpserver)

- Goal: Make the best HTTP File Server.
- Features: Human-friendly UI, file uploading support, direct QR-code generation for Apple & Android install package.

## Notes
在原有基础上增加创建folder,删除folder,编辑文本的feature.

Upload size now is limited to 1G.

## Features
1. [x] make directory
1. [x] delete directory
1. [X] Edit file support (only for text file)
1. [X] Simple directory checkout support (only single file could be marked as checked in ".ghs.yml" file or will checkout lastest modifed file in directory. Directory will be used as channel for static file management and dynamic redirect. Could be accessed by: `GET <IP>:<PORT>/-/checkout/directory`)

## Advanced usage
Add access rule by creating a `.ghs.yml` file under a sub-directory. An example:

```yaml
---
upload: false
delete: false
mkdir: false
checked: "world.txt"
users:
- email: "codeskyblue@codeskyblue.com"
  delete: true
  upload: true
  mkdir: true
```

In this case, if openid auth is enabled and user "codeskyblue@codeskyblue.com" has logged in, he/she can delete/upload files under the directory where the `.ghs.yml` file exits.

For example, in the following directory hierarchy, users can delete/uploade files in directory `foo`, but he/she cannot do this in directory `bar`.

```
root -
  |-- foo
  |    |-- .ghs.yml
  |    `-- world.txt 
  `-- bar
       `-- hello.txt
```

User can specify config file name with `--conf`, see [example config.yml](testdata/config.yml).

To specify which files is hidden and which file is visible, add the following lines to `.ghs.yml`

```yaml
accessTables:
- regex: block.file
  allow: false
- regex: visual.file
  allow: true
```

### ipa plist proxy
This is used for server on which https is enabled. default use <https://plistproxy.herokuapp.com/plist>

```
./gohttpserver --plistproxy=https://someproxyhost.com/
```

Test if proxy works:

```sh
$ http POST https://proxyhost.com/plist < app.plist
{
	"key": "18f99211"
}
$ http GET https://proxyhost.com/plist/18f99211
# show the app.plist content
```

### Upload with CURL
For example, upload a file named `foo.txt` to directory `somedir`
(the maximum upload file size is hard coded and limited to 1 GB)

```sh
$ curl -F file=@foo.txt localhost:8000/somedir
```
## LICENSE
This project is licensed under [MIT](LICENSE).

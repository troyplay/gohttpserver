## 功能说明
1. 权限管理与配置文件
	- 静态文件管理，删除，上传，下载，创建子目录
	- 目录权限配置
1. chanel与checkout
	- 将目录作为一个channel，对资源文件进行静态存储与动态redirect
	- checkout channel将返回目录中配置文件指定标签的文件或者默认的最新上传/修改的文件

## 系统配置文件
在根目录中有一个配置文件`config.yml`,里面配置系统的启动项
example:

```yaml
---
addr: ":4000" // 端口
title: "Model管理" // title
theme: black // 页面主题
debug: true // debug 模式
xheaders: true
cors: true //跨域支持
root: filesdata/ //文件系统根目录

```
更多请参考GitHub: `gohttpserver `, [gohttpserver](https://github.com/troyplay/gohttpserver).

## 权限管理与配置文件
在每个目录中有一个隐藏文件`.ghs.yml`,里面配置文件权限表和操作权限
example:

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
accessTables:
- regex: block.file
  allow: false
- regex: visual.file
  allow: true

```

| 字段      | 类型           | 说明  |
| ------------- |:-------------:| :-----:|
| **upload**  | bool | 是否可以在当前目录中上传文件 |
| **delete**     | bool   |   是否可以在当前目录中删除文件或者子目录|
| **mkdir**  | bool  |    是否可以在当前目录中创建子目录|
| **checked**  | string  |    如果当前目录作为channel，checked指定当前channel活跃的文件|
| **users**  | table  |   针对每一个用户进行对应的权限配置|
| **accessTables**  | table  |   针对每一个文件配置是否可见|
 
 - users 表:

| 字段      | 类型           | 说明  |
| ------------- |:-------------:| :-----:|
| **email**  | string | user id |
| **upload**  | bool | 是否可以在当前目录中上传文件 |
| **delete**     | bool   |   是否可以在当前目录中删除文件或者子目录|
| **mkdir**  | bool  |    是否可以在当前目录中创建子目录|

 - accessTables 表:

| 字段      | 类型           | 说明  |
| ------------- |:-------------:| :-----:|
| ** regex**  | string | 文件匹配正则表达式 |
| **upload**  | bool | 是否可以在当前目录中上传文件 |
| **allow**     | bool   |   文件是否可见|

## channel与checkout 
通过设置目录的checked配置项，可以改变channel的checkout资源地址。默认返回目录中最新上传或修改的文件。
```
GET <IP>:<PORT>/-/checkout/<directory>
```
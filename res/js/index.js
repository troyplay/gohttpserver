jQuery('#qrcodeCanvas').qrcode({
  text: "http://jetienne.com/"
});

Dropzone.autoDiscover = false;

function getExtention(fname) {
  return fname.slice((fname.lastIndexOf(".") - 1 >>> 0) + 2);
}

function pathJoin(parts, sep) {
  var separator = sep || '/';
  var replace = new RegExp(separator + '{1,}', 'g');
  return parts.join(separator).replace(replace, separator);
}

function getQueryString(name) {
  var reg = new RegExp("(^|&)" + name + "=([^&]*)(&|$)");
  var r = decodeURI(window.location.search).substr(1).match(reg);
  if (r != null) return r[2].replace(/\+/g, ' ');
  return null;
}

var vm = new Vue({
  el: "#app",
  data: {
    user: {
      email: "",
      name: "",
    },
    location: window.location,
    breadcrumb: [],
    showHidden: false,
    previewMode: false,
    preview: {
      filename: '',
      filetype: '',
      filesize: 0,
      contentHTML: '',
    },
    version: "loading",
    mtimeTypeFromNow: false, // or fromNow
    auth: {},
    search: getQueryString("search"),
    files: [{
      name: "loading ...",
      path: "",
      size: "...",
      type: "dir",
    }],
    myDropzone: null,
  },
  computed: {
    computedFiles: function() {
      var that = this;
      that.preview.filename = null;

      var files = this.files.filter(function(f) {
        if (f.name == 'README.md') {
          that.preview.filename = f.name;
        }
        if (!that.showHidden && f.name.slice(0, 1) === '.') {
          return false;
        }
        return true;
      });
      // console.log(this.previewFile)
      if (this.preview.filename) {
        var name = this.preview.filename; // For now only README.md
        console.log(pathJoin([location.pathname, 'README.md']))
        $.ajax({
          url: pathJoin([location.pathname, 'README.md']),
          method: 'GET',
          success: function(res) {
            var converter = new showdown.Converter({
              tables: true,
              omitExtraWLInCodeBlocks: true,
              parseImgDimensions: true,
              simplifiedAutoLink: true,
              literalMidWordUnderscores: true,
              tasklists: true,
              ghCodeBlocks: true,
              smoothLivePreview: true,
              simplifiedAutoLink: true,
              strikethrough: true,
            });

            var html = converter.makeHtml(res);
            that.preview.contentHTML = html;
          },
          error: function(err) {
            console.log(err)
          }
        })
      }

      return files;
    },
  },
  created: function() {
    $.ajax({
      url: "/-/user",
      method: "get",
      dataType: "json",
      success: function(ret) {
        if (ret) {
          this.user.email = ret.email;
          this.user.name = ret.name;
        }
      }.bind(this)
    })
    this.myDropzone = new Dropzone("#upload-form", {
      paramName: "file",
      maxFilesize: 1024,
      addRemoveLinks: true,
      init: function() {
        this.on("uploadprogress", function(file, progress) {
          // console.log("File progress", progress);
        });
        this.on("complete", function(file) {
          console.log("reload file list")
          loadFileList()
        })
      }
    });
  },
  methods: {
    formatTime: function(timestamp) {
      var m = moment(timestamp);
      if (this.mtimeTypeFromNow) {
        return m.fromNow();
      }
      return m.format('YYYY-MM-DD HH:mm:ss');
    },
    toggleHidden: function() {
      this.showHidden = !this.showHidden;
    },
    removeAllUploads: function() {
      this.myDropzone.removeAllFiles();
    },
    clickMkdir: function(){
      var folderName = $("#mkdir-input").val();
      if (folderName != "") {
        $.ajax({
          url: pathJoin(["/-/mkdir", location.pathname]),
          dataType:"text",
          method: "POST",
          data:{"folderName" : folderName},
          success: function(res) {
            console.log("mkdir successfully");
            // clear input
            $("#mkdir-input").val('');
            $('#mkdir-modal').modal("hide");
            // reload files
            loadFileList();
          },
          error:function(res){
            console.error(res);
            $('#mkdir-error-alert span').text(res.responseText);
            $('#mkdir-error-alert').fadeIn('slow').delay(3000).fadeOut('slow');
          }
        })
      }else{
        $('#mkdir-error-alert span').text('Folder Name can\'t be empty！');
        $('#mkdir-error-alert').fadeIn('slow').delay(3000).fadeOut('slow');
      }
    },
    // load file content and edit file
    showEditFile: function(f) {
      $.ajax({
        url: pathJoin([location.pathname, f.name]),
        method: "GET",
        success: function(res) {
          $("#file-edit-title").text(f.name);
          $("#file-edit-text-area").val(res);
          $("#file-edit-modal").modal("show");
        },
        error:function(res){
          console.error(res);
          $('#edit-error-alert span').text(res.responseText);
          $('#edit-error-alert').fadeIn('slow').delay(3000).fadeOut('slow');
        }
      })
    },
    // save edit file content
    saveEditFile: function() {
      var fileContent = $("#file-edit-text-area").val();
      var fileName = $("#file-edit-title").text();
      $.ajax({
          url: pathJoin([location.pathname, fileName]),
          dataType: "text",
          method: "PUT",
          data:{"content" : fileContent},
          success: function(res) {
            $("#file-edit-modal").modal("hide");
            // reload files
            loadFileList();
          },
          error:function(res){
            console.error(res);
            $('#edit-error-alert span').text(res.responseText);
            $('#edit-error-alert').fadeIn('slow').delay(3000).fadeOut('slow');
          }
        })
    },
    genInstallURL: function(name) {
      var urlPath;
      if (getExtention(name) == "ipa") {
        urlPath = location.protocol + "//" + pathJoin([location.host, "/-/ipa/link", location.pathname, name]);
      } else {
        urlPath = location.protocol + "//" + pathJoin([location.host, location.pathname, name]);
      }
      return encodeURI(urlPath);
    },
    genQrcode: function(name, title) {
      var installURL = this.genInstallURL(name);
      $("#qrcode-title").html(title || name);
      $("#qrcode-link").attr("href", installURL);
      $('#qrcodeCanvas').empty().qrcode({
        text: installURL
      });
      $("#qrcodeRight a").attr("href", encodeURI(location.origin + pathJoin([location.pathname, name])));
      $("#qrcode-modal").modal("show");
    },
    genDownloadURL: function(f) {
      return location.origin + "/" + f.path;
    },
    shouldHaveQrcode: function(name) {
      return ['apk', 'ipa'].indexOf(getExtention(name)) !== -1;
    },
    // whether file is editable
    isFileEditable: function(name) {
      return name.endsWith('.yml');
    },
    genFileClass: function(f) {
      if (f.type == "dir") {
        if (f.name == '.git') {
          return 'fa-git-square';
        }
        return "fa-folder-open";
      }
      var ext = getExtention(f.name);
      switch (ext) {
        case "go":
        case "py":
        case "js":
        case "java":
        case "c":
        case "cpp":
        case "h":
          return "fa-file-code-o";
        case "pdf":
          return "fa-file-pdf-o";
        case "zip":
          return "fa-file-zip-o";
        case "mp3":
        case "wav":
          return "fa-file-audio-o";
        case "jpg":
        case "png":
        case "gif":
        case "jpeg":
        case "tiff":
          return "fa-file-picture-o";
        case "ipa":
        case "dmg":
          return "fa-apple";
        case "apk":
          return "fa-android";
        case "exe":
          return "fa-windows";
      }
      return "fa-file-text-o"
    },
    clickFileOrDir: function(f, e) {
      // TODO: fix here tomorrow
      if (f.type == "file") {
        return true;
      }
      var reqPath = pathJoin([location.pathname, f.name]);
      loadFileOrDir(reqPath);
      e.preventDefault()
    },
    changePath: function(reqPath, e) {
      loadFileOrDir(reqPath);
      e.preventDefault()
    },
    showInfo: function(f) {
      console.log(f);
      $.ajax({
        url: pathJoin(["/-/info", location.pathname, f.name]),
        method: "GET",
        success: function(res) {
          $("#file-info-title").text(f.name);
          $("#file-info-content").text(JSON.stringify(res, null, 4));
          $("#file-info-modal").modal("show");
          // console.log(JSON.stringify(res, null, 4));
        }
      })
    },
    deletePathConfirm: function(f, e) {
      // confirm
      e.preventDefault();
      if(!confirm("确认删除？")){
        return;
      }
      $.ajax({
        url: pathJoin([location.pathname, f.name]),
        method: 'DELETE',
        success: function(res) {
          loadFileList()
        },
        error: function(err) {
          alert(err.responseText);
        }
      });
    },
    updateBreadcrumb: function() {
      var pathname = decodeURI(location.pathname || "/");
      var parts = pathname.split('/');
      this.breadcrumb = [];
      if (pathname == "/") {
        return this.breadcrumb;
      }
      var i = 2;
      for (; i <= parts.length; i += 1) {
        var name = parts[i - 1];
        if (!name) {
          continue;
        }
        var path = parts.slice(0, i).join('/');
        this.breadcrumb.push({
          name: name + (i == parts.length ? ' /' : ''),
          path: path
        })
      }
      return this.breadcrumb;
    },
    loadPreviewFile: function(filepath, e) {
      if (e) {
        e.preventDefault() // may be need a switch
      }
      var that = this;
      $.getJSON(pathJoin(['/-/info', location.pathname]))
        .then(function(res) {
          console.log(res);
          that.preview.filename = res.name;
          that.preview.filesize = res.size;
          return $.ajax({
            url: '/' + res.path,
            dataType: 'text',
          });
        })
        .then(function(res) {
          console.log(res)
          that.preview.contentHTML = '<pre>' + res + '</pre>';
          console.log("Finally")
        })
        .done(function(res) {
          console.log("done", res)
        });
    },
    loadAll: function() {
      // TODO: move loadFileList here
    },
  }
})

window.onpopstate = function(event) {
  var pathname = decodeURI(location.pathname)
  loadFileList()
}

function loadFileOrDir(reqPath) {
  window.history.pushState({}, "", reqPath);
  loadFileList(reqPath)
}

function loadFileList(pathname) {
  var pathname = pathname || location.pathname;
  // console.log("load filelist:", pathname)
  if (getQueryString("raw") !== "false") { // not a file preview
    $.ajax({
      url: pathJoin(["/-/json", pathname]),
      dataType: "json",
      cache: false,
      success: function(res) {
        res.files = _.sortBy(res.files, function(f) {
          var weight = f.type == 'dir' ? 1000 : 1;
          return -weight * f.mtime;
        })

        vm.files = res.files;
        vm.auth = res.auth;
      },
      error: function(err) {
        console.error(err)
      },
    });

  }

  vm.updateBreadcrumb();
  vm.previewMode = getQueryString("raw") == "false";
  if (vm.previewMode) {
    vm.loadPreviewFile();
  }
}

Vue.filter('fromNow', function(value) {
  return moment(value).fromNow();
})

Vue.filter('formatBytes', function(value) {
  var bytes = parseFloat(value);
  if (bytes < 0) return "-";
  else if (bytes < 1024) return bytes + " B";
  else if (bytes < 1048576) return (bytes / 1024).toFixed(0) + " KB";
  else if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + " MB";
  else return (bytes / 1073741824).toFixed(1) + " GB";
})

$(function() {
  $.scrollUp({
    scrollText: '', // text are defined in css
  });

  // For page first loading
  loadFileList(location.pathname + location.search)

  // update version
  $.getJSON("/-/sysinfo", function(res) {
    vm.version = res.version;
  })

  var clipboard = new Clipboard('.btn');
  clipboard.on('success', function(e) {
    console.info('Action:', e.action);
    console.info('Text:', e.text);
    console.info('Trigger:', e.trigger);
    $(e.trigger)
      .tooltip('show')
      .mouseleave(function() {
        $(this).tooltip('hide');
      })

    e.clearSelection();
  });
});

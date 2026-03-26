package jsworker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/pkg/pluginhost"
	"auralogic/internal/pkg/pluginutil"
	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

const (
	workerVersion                     = "js-worker/1.1.0"
	workerRequestReadTimeout          = 5 * time.Second
	workerWriteTimeout                = 5 * time.Second
	defaultStorageMaxKeys             = 512
	defaultStorageMaxTotalBytes       = int64(4 * 1024 * 1024)
	defaultStorageMaxValueBytes       = int64(64 * 1024)
	defaultStorageMaxKeyBytes         = 191
	defaultMaxMemoryMB                = 128
	memoryMonitorInterval             = 10 * time.Millisecond
	memoryMonitorMinInterval          = 50 * time.Millisecond
	memoryMonitorMaxInterval          = 250 * time.Millisecond
	memoryBreakerThreshold            = 3
	memoryBreakerCooldown             = 30 * time.Second
	defaultPluginHTTPTimeoutMs        = 10000
	maxPluginHTTPResponseBytes        = int64(1024 * 1024)
	maxPluginHTTPRedirects            = 5
	pluginHTTPUserAgent               = "AuraLogic-PluginRuntime/1.0"
	storageAccessMetaKey              = "storage_access_mode"
	storageAccessNone                 = "none"
	storageAccessRead                 = "read"
	storageAccessWrite                = "write"
	workerWorkspaceHostInputTimeoutMs = 15 * 60 * 1000
)

const urlSearchParamsPolyfillSource = `
(function () {
  if (typeof URLSearchParams !== "undefined") {
    return;
  }

  var hasOwn = Object.prototype.hasOwnProperty;

  function decodePart(value) {
    return decodeURIComponent(String(value || "").replace(/\+/g, " "));
  }

  function encodePart(value) {
    return encodeURIComponent(String(value == null ? "" : value)).replace(/%20/g, "+");
  }

  function URLSearchParamsPolyfill(init) {
    this._pairs = [];
    if (init == null) {
      return;
    }
    if (typeof init === "string") {
      this._fromString(init);
      return;
    }
    if (Array.isArray(init)) {
      for (var i = 0; i < init.length; i += 1) {
        var pair = init[i];
        if (!pair || pair.length < 2) {
          continue;
        }
        this.append(pair[0], pair[1]);
      }
      return;
    }
    if (typeof init === "object") {
      for (var key in init) {
        if (!hasOwn.call(init, key)) {
          continue;
        }
        var current = init[key];
        if (Array.isArray(current)) {
          for (var j = 0; j < current.length; j += 1) {
            this.append(key, current[j]);
          }
          continue;
        }
        this.append(key, current);
      }
    }
  }

  URLSearchParamsPolyfill.prototype._fromString = function (input) {
    var source = String(input || "");
    if (!source) {
      return;
    }
    if (source.charAt(0) === "?") {
      source = source.slice(1);
    }
    if (!source) {
      return;
    }
    var parts = source.split("&");
    for (var i = 0; i < parts.length; i += 1) {
      var item = parts[i];
      if (!item) {
        continue;
      }
      var index = item.indexOf("=");
      if (index < 0) {
        this.append(decodePart(item), "");
        continue;
      }
      this.append(decodePart(item.slice(0, index)), decodePart(item.slice(index + 1)));
    }
  };

  URLSearchParamsPolyfill.prototype.append = function (key, value) {
    this._pairs.push([String(key), String(value == null ? "" : value)]);
  };

  URLSearchParamsPolyfill.prototype.set = function (key, value) {
    var normalizedKey = String(key);
    var normalizedValue = String(value == null ? "" : value);
    var next = [];
    var replaced = false;
    for (var i = 0; i < this._pairs.length; i += 1) {
      var pair = this._pairs[i];
      if (pair[0] === normalizedKey) {
        if (!replaced) {
          next.push([normalizedKey, normalizedValue]);
          replaced = true;
        }
        continue;
      }
      next.push(pair);
    }
    if (!replaced) {
      next.push([normalizedKey, normalizedValue]);
    }
    this._pairs = next;
  };

  URLSearchParamsPolyfill.prototype.get = function (key) {
    var normalizedKey = String(key);
    for (var i = 0; i < this._pairs.length; i += 1) {
      if (this._pairs[i][0] === normalizedKey) {
        return this._pairs[i][1];
      }
    }
    return null;
  };

  URLSearchParamsPolyfill.prototype.getAll = function (key) {
    var normalizedKey = String(key);
    var result = [];
    for (var i = 0; i < this._pairs.length; i += 1) {
      if (this._pairs[i][0] === normalizedKey) {
        result.push(this._pairs[i][1]);
      }
    }
    return result;
  };

  URLSearchParamsPolyfill.prototype.has = function (key) {
    return this.get(key) !== null;
  };

  URLSearchParamsPolyfill.prototype.delete = function (key) {
    var normalizedKey = String(key);
    this._pairs = this._pairs.filter(function (pair) {
      return pair[0] !== normalizedKey;
    });
  };

  URLSearchParamsPolyfill.prototype.forEach = function (callback, thisArg) {
    for (var i = 0; i < this._pairs.length; i += 1) {
      var pair = this._pairs[i];
      callback.call(thisArg, pair[1], pair[0], this);
    }
  };

  URLSearchParamsPolyfill.prototype.toString = function () {
    var parts = [];
    for (var i = 0; i < this._pairs.length; i += 1) {
      parts.push(encodePart(this._pairs[i][0]) + "=" + encodePart(this._pairs[i][1]));
    }
    return parts.join("&");
  };

  if (typeof globalThis !== "undefined") {
    globalThis.URLSearchParams = URLSearchParamsPolyfill;
  } else {
    this.URLSearchParams = URLSearchParamsPolyfill;
  }
})();
`

const webEncodingPolyfillSource = `
(function () {
  var root = typeof globalThis !== "undefined" ? globalThis : this;

  function defineValue(name, value) {
    if (typeof root[name] === "undefined") {
      root[name] = value;
    }
  }

  function normalizeString(input) {
    return String(input == null ? "" : input);
  }

  function nextCodePoint(input, index) {
    var first = input.charCodeAt(index);
    var nextIndex = index + 1;
    if (first >= 55296 && first <= 56319 && nextIndex < input.length) {
      var second = input.charCodeAt(nextIndex);
      if ((second & 64512) === 56320) {
        return {
          codePoint: ((first - 55296) << 10) + (second - 56320) + 65536,
          nextIndex: nextIndex + 1
        };
      }
    }
    return {
      codePoint: first,
      nextIndex: nextIndex
    };
  }

  function encodeCodePoint(codePoint, bytes) {
    if (codePoint <= 127) {
      bytes.push(codePoint);
      return;
    }
    if (codePoint <= 2047) {
      bytes.push(192 | (codePoint >> 6));
      bytes.push(128 | (codePoint & 63));
      return;
    }
    if (codePoint <= 65535) {
      bytes.push(224 | (codePoint >> 12));
      bytes.push(128 | ((codePoint >> 6) & 63));
      bytes.push(128 | (codePoint & 63));
      return;
    }
    bytes.push(240 | (codePoint >> 18));
    bytes.push(128 | ((codePoint >> 12) & 63));
    bytes.push(128 | ((codePoint >> 6) & 63));
    bytes.push(128 | (codePoint & 63));
  }

  function utf8Encode(input) {
    var text = normalizeString(input);
    var bytes = [];
    var index = 0;
    while (index < text.length) {
      var item = nextCodePoint(text, index);
      encodeCodePoint(item.codePoint, bytes);
      index = item.nextIndex;
    }
    return bytes;
  }

  function utf8Decode(bytes, fatal) {
    var output = [];
    var index = 0;

    function fail() {
      if (fatal) {
        throw new TypeError("Invalid UTF-8 data");
      }
      output.push(String.fromCharCode(65533));
    }

    while (index < bytes.length) {
      var byte1 = bytes[index++];
      if (byte1 <= 127) {
        output.push(String.fromCharCode(byte1));
        continue;
      }

      var needed = 0;
      var min = 0;
      var codePoint = 0;
      if (byte1 >= 194 && byte1 <= 223) {
        needed = 1;
        min = 128;
        codePoint = byte1 & 31;
      } else if (byte1 >= 224 && byte1 <= 239) {
        needed = 2;
        min = 2048;
        codePoint = byte1 & 15;
      } else if (byte1 >= 240 && byte1 <= 244) {
        needed = 3;
        min = 65536;
        codePoint = byte1 & 7;
      } else {
        fail();
        continue;
      }

      if (index + needed > bytes.length) {
        fail();
        break;
      }

      var valid = true;
      for (var i = 0; i < needed; i += 1) {
        var next = bytes[index++];
        if ((next & 192) !== 128) {
          valid = false;
          index -= 1;
          break;
        }
        codePoint = (codePoint << 6) | (next & 63);
      }
      if (!valid || codePoint < min || codePoint > 1114111 || (codePoint >= 55296 && codePoint <= 57343)) {
        fail();
        continue;
      }

      if (codePoint <= 65535) {
        output.push(String.fromCharCode(codePoint));
        continue;
      }

      codePoint -= 65536;
      output.push(String.fromCharCode(55296 + (codePoint >> 10)));
      output.push(String.fromCharCode(56320 + (codePoint & 1023)));
    }

    return output.join("");
  }

  function toByteArray(input) {
    if (input == null) {
      return [];
    }
    if (typeof input === "string") {
      var direct = [];
      for (var i = 0; i < input.length; i += 1) {
        direct.push(input.charCodeAt(i) & 255);
      }
      return direct;
    }
    if (typeof ArrayBuffer !== "undefined" && input instanceof ArrayBuffer) {
      if (typeof Uint8Array !== "undefined") {
        return toByteArray(new Uint8Array(input));
      }
      return [];
    }
    if (Array.isArray(input)) {
      return input.map(function (item) {
        return Number(item) & 255;
      });
    }
    if (typeof input.length === "number") {
      var bytes = [];
      for (var j = 0; j < input.length; j += 1) {
        bytes.push(Number(input[j]) & 255);
      }
      return bytes;
    }
    if (typeof ArrayBuffer !== "undefined" && input && input.buffer && typeof input.byteLength === "number") {
      if (typeof Uint8Array !== "undefined") {
        return toByteArray(new Uint8Array(input.buffer, input.byteOffset || 0, input.byteLength));
      }
    }
    return [];
  }

  function toUint8Array(bytes) {
    if (typeof Uint8Array !== "undefined") {
      return new Uint8Array(bytes);
    }
    return bytes.slice();
  }

  if (typeof root.TextEncoder === "undefined") {
    function TextEncoderPolyfill() {}
    TextEncoderPolyfill.prototype.encoding = "utf-8";
    TextEncoderPolyfill.prototype.encode = function (input) {
      return toUint8Array(utf8Encode(input));
    };
    TextEncoderPolyfill.prototype.encodeInto = function (input, destination) {
      var text = normalizeString(input);
      if (!destination || typeof destination.length !== "number") {
        return { read: 0, written: 0 };
      }
      var index = 0;
      var read = 0;
      var written = 0;
      while (index < text.length) {
        var item = nextCodePoint(text, index);
        var bytes = [];
        encodeCodePoint(item.codePoint, bytes);
        if (written + bytes.length > destination.length) {
          break;
        }
        for (var i = 0; i < bytes.length; i += 1) {
          destination[written + i] = bytes[i];
        }
        written += bytes.length;
        read = item.nextIndex;
        index = item.nextIndex;
      }
      return { read: read, written: written };
    };
    defineValue("TextEncoder", TextEncoderPolyfill);
  }

  if (typeof root.TextDecoder === "undefined") {
    function TextDecoderPolyfill(label, options) {
      var normalizedLabel = label == null ? "utf-8" : String(label).toLowerCase();
      if (normalizedLabel && normalizedLabel !== "utf-8" && normalizedLabel !== "utf8") {
        throw new RangeError("Only utf-8 is supported");
      }
      this.encoding = "utf-8";
      this.fatal = !!(options && options.fatal);
      this.ignoreBOM = !!(options && options.ignoreBOM);
    }
    TextDecoderPolyfill.prototype.decode = function (input) {
      var bytes = toByteArray(input);
      if (!this.ignoreBOM && bytes.length >= 3 && bytes[0] === 239 && bytes[1] === 187 && bytes[2] === 191) {
        bytes = bytes.slice(3);
      }
      return utf8Decode(bytes, this.fatal);
    };
    defineValue("TextDecoder", TextDecoderPolyfill);
  }

  var base64abc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

  function btoaPolyfill(input) {
    var source = normalizeString(input);
    var output = "";
    var index = 0;
    while (index < source.length) {
      var byte1 = source.charCodeAt(index++);
      if (byte1 > 255) {
        throw new TypeError("btoa only supports binary strings");
      }
      var has2 = index < source.length;
      var byte2 = has2 ? source.charCodeAt(index++) : 0;
      if (has2 && byte2 > 255) {
        throw new TypeError("btoa only supports binary strings");
      }
      var has3 = index < source.length;
      var byte3 = has3 ? source.charCodeAt(index++) : 0;
      if (has3 && byte3 > 255) {
        throw new TypeError("btoa only supports binary strings");
      }
      var triplet = (byte1 << 16) | (byte2 << 8) | byte3;
      output += base64abc.charAt((triplet >> 18) & 63);
      output += base64abc.charAt((triplet >> 12) & 63);
      output += has2 ? base64abc.charAt((triplet >> 6) & 63) : "=";
      output += has3 ? base64abc.charAt(triplet & 63) : "=";
    }
    return output;
  }

  function atobPolyfill(input) {
    var source = normalizeString(input).replace(/[\t\n\f\r ]+/g, "");
    if (source.length % 4 === 1) {
      throw new TypeError("Invalid base64 input");
    }
    var output = [];
    var index = 0;
    while (index < source.length) {
      var char1 = source.charAt(index++);
      var char2 = source.charAt(index++);
      var char3 = source.charAt(index++);
      var char4 = source.charAt(index++);
      var enc1 = base64abc.indexOf(char1);
      var enc2 = base64abc.indexOf(char2);
      var enc3 = char3 === "=" ? 64 : base64abc.indexOf(char3);
      var enc4 = char4 === "=" ? 64 : base64abc.indexOf(char4);
      if (enc1 < 0 || enc2 < 0 || enc3 < 0 || enc4 < 0) {
        throw new TypeError("Invalid base64 input");
      }
      var triplet = (enc1 << 18) | (enc2 << 12) | ((enc3 & 63) << 6) | (enc4 & 63);
      output.push(String.fromCharCode((triplet >> 16) & 255));
      if (enc3 !== 64) {
        output.push(String.fromCharCode((triplet >> 8) & 255));
      }
      if (enc4 !== 64) {
        output.push(String.fromCharCode(triplet & 255));
      }
    }
    return output.join("");
  }

  defineValue("btoa", btoaPolyfill);
  defineValue("atob", atobPolyfill);
})();
`

var pluginHTTPCGNATPrefix = netip.MustParsePrefix("100.64.0.0/10")

var sharedPluginHTTPTransport = &http.Transport{
	Proxy:               http.ProxyFromEnvironment,
	MaxIdleConns:        32,
	MaxIdleConnsPerHost: 8,
	IdleConnTimeout:     30 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
	ForceAttemptHTTP2:   true,
	DialContext:         pluginHTTPDialContext,
}

var pluginHostSessions sync.Map

type pluginHostSession struct {
	mu          sync.Mutex
	network     string
	address     string
	accessToken string
	conn        net.Conn
	encoder     *json.Encoder
	decoder     *json.Decoder
}

type workerOptions struct {
	network              string
	socket               string
	timeoutMs            int
	maxConcurrency       int
	maxMemoryMB          int
	allowNetwork         bool
	allowFS              bool
	artifactRoot         string
	fsMaxFiles           int
	fsMaxTotalBytes      int64
	fsMaxReadBytes       int64
	storageMaxKeys       int
	storageMaxTotalBytes int64
	storageMaxValueBytes int64
}

func Run(args []string) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.socket) == "" {
		return fmt.Errorf("jsworker: -socket is required")
	}
	if opts.timeoutMs <= 0 {
		opts.timeoutMs = 30000
	}
	if opts.maxConcurrency <= 0 {
		opts.maxConcurrency = 4
	}
	if opts.maxMemoryMB <= 0 {
		opts.maxMemoryMB = defaultMaxMemoryMB
	}
	opts.network = strings.ToLower(strings.TrimSpace(opts.network))
	if opts.network == "" {
		opts.network = "unix"
	}
	if opts.fsMaxFiles <= 0 {
		opts.fsMaxFiles = 2048
	}
	if opts.fsMaxTotalBytes <= 0 {
		opts.fsMaxTotalBytes = 128 * 1024 * 1024
	}
	if opts.fsMaxReadBytes <= 0 {
		opts.fsMaxReadBytes = 4 * 1024 * 1024
	}
	if opts.fsMaxReadBytes > opts.fsMaxTotalBytes {
		opts.fsMaxReadBytes = opts.fsMaxTotalBytes
	}
	if opts.storageMaxKeys <= 0 {
		opts.storageMaxKeys = defaultStorageMaxKeys
	}
	if opts.storageMaxTotalBytes <= 0 {
		opts.storageMaxTotalBytes = defaultStorageMaxTotalBytes
	}
	if opts.storageMaxValueBytes <= 0 {
		opts.storageMaxValueBytes = defaultStorageMaxValueBytes
	}
	if opts.storageMaxValueBytes > opts.storageMaxTotalBytes {
		opts.storageMaxValueBytes = opts.storageMaxTotalBytes
	}
	opts.artifactRoot = filepath.Clean(filepath.FromSlash(strings.TrimSpace(opts.artifactRoot)))
	if opts.artifactRoot == "" || opts.artifactRoot == "." {
		opts.artifactRoot = filepath.Join("data", "plugins")
	}
	if !filepath.IsAbs(opts.artifactRoot) {
		if absRoot, absErr := filepath.Abs(opts.artifactRoot); absErr == nil {
			opts.artifactRoot = filepath.Clean(absRoot)
		}
	}
	if err := pluginutil.ValidateJSWorkerSocketEndpoint(opts.network, opts.socket); err != nil {
		return fmt.Errorf("jsworker: invalid listener endpoint: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(opts.artifactRoot, "data"), 0755); err != nil {
		return fmt.Errorf("jsworker: create artifact data dir failed: %w", err)
	}

	if opts.network == "unix" {
		_ = os.Remove(opts.socket)
		if err := os.MkdirAll(filepath.Dir(opts.socket), 0755); err != nil {
			return fmt.Errorf("jsworker: create socket dir failed: %w", err)
		}
	}

	listener, err := net.Listen(opts.network, opts.socket)
	if err != nil {
		return fmt.Errorf("jsworker: listen failed: %w", err)
	}
	defer listener.Close()
	if opts.network == "unix" {
		defer os.Remove(opts.socket)
	}

	log.Printf(
		"jsworker started network=%s socket=%s timeout_ms=%d max_concurrency=%d max_memory_mb=%d allow_network=%t allow_fs=%t artifact_root=%s fs_max_files=%d fs_max_total_bytes=%d fs_max_read_bytes=%d storage_max_keys=%d storage_max_total_bytes=%d storage_max_value_bytes=%d",
		opts.network,
		opts.socket,
		opts.timeoutMs,
		opts.maxConcurrency,
		opts.maxMemoryMB,
		opts.allowNetwork,
		opts.allowFS,
		filepath.ToSlash(opts.artifactRoot),
		opts.fsMaxFiles,
		opts.fsMaxTotalBytes,
		opts.fsMaxReadBytes,
		opts.storageMaxKeys,
		opts.storageMaxTotalBytes,
		opts.storageMaxValueBytes,
	)

	semaphore := make(chan struct{}, opts.maxConcurrency)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Printf("jsworker accept temporary error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("jsworker accept error: %w", err)
		}

		semaphore <- struct{}{}
		go func(c net.Conn) {
			defer func() { <-semaphore }()
			handleConnection(c, opts)
		}(conn)
	}
}

func parseFlags(args []string) (workerOptions, error) {
	var opts workerOptions
	fs := flag.NewFlagSet("js-worker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.network, "network", "unix", "listen network: unix or tcp")
	fs.StringVar(&opts.socket, "socket", "", "socket path (unix) or address (tcp)")
	fs.IntVar(&opts.timeoutMs, "timeout-ms", 30000, "default execute timeout in milliseconds")
	fs.IntVar(&opts.maxConcurrency, "max-concurrency", 4, "max concurrent requests")
	fs.IntVar(&opts.maxMemoryMB, "max-memory-mb", defaultMaxMemoryMB, "max heap growth allowed per request (in MB)")
	fs.BoolVar(&opts.allowNetwork, "allow-network", false, "allow network-related APIs (policy hint)")
	fs.BoolVar(&opts.allowFS, "allow-fs", false, "allow filesystem-related APIs (policy hint)")
	fs.StringVar(&opts.artifactRoot, "artifact-root", filepath.Join("data", "plugins"), "plugin artifact root directory")
	fs.IntVar(&opts.fsMaxFiles, "fs-max-files", 2048, "max files allowed inside plugin filesystem root")
	fs.Int64Var(&opts.fsMaxTotalBytes, "fs-max-total-bytes", 128*1024*1024, "max total bytes allowed inside plugin filesystem root")
	fs.Int64Var(&opts.fsMaxReadBytes, "fs-max-read-bytes", 4*1024*1024, "max bytes allowed for one fs read call")
	fs.IntVar(&opts.storageMaxKeys, "storage-max-keys", defaultStorageMaxKeys, "max keys allowed in Plugin.storage")
	fs.Int64Var(&opts.storageMaxTotalBytes, "storage-max-total-bytes", defaultStorageMaxTotalBytes, "max total bytes allowed in Plugin.storage")
	fs.Int64Var(&opts.storageMaxValueBytes, "storage-max-value-bytes", defaultStorageMaxValueBytes, "max bytes allowed for one Plugin.storage value")
	if err := fs.Parse(args); err != nil {
		return workerOptions{}, err
	}
	return opts, nil
}

func handleConnection(conn net.Conn, opts workerOptions) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(workerRequestReadTimeout))

	decoder := json.NewDecoder(conn)
	var req pluginipc.Request
	if err := decoder.Decode(&req); err != nil {
		writeResponse(conn, pluginipc.Response{
			Success: false,
			Error:   fmt.Sprintf("invalid request payload: %v", err),
		})
		return
	}
	_ = conn.SetDeadline(time.Time{})

	var resp pluginipc.Response
	switch strings.ToLower(strings.TrimSpace(req.Type)) {
	case "health":
		resp = handleHealthRequest(req, opts)
	case "runtime_state":
		resp = handleRuntimeStateRequest(req)
	case "dispose_runtime":
		globalPersistentPluginRuntimeManager.dispose(req.PluginID, req.PluginGeneration)
		resp = pluginipc.Response{
			Success: true,
			Healthy: true,
			Version: workerVersion,
		}
	case "execute":
		resp = handleExecuteRequest(conn, req, opts)
	case "runtime_eval":
		resp = handleRuntimeEvalRequest(conn, req, opts)
	case "runtime_inspect":
		resp = handleRuntimeInspectRequest(conn, req, opts)
	case "execute_stream":
		handleExecuteStreamRequest(conn, req, opts)
		return
	default:
		resp = pluginipc.Response{
			Success: false,
			Error:   fmt.Sprintf("unsupported request type %q", req.Type),
		}
	}

	writeResponse(conn, resp)
}

func encodeResponse(w io.Writer, resp pluginipc.Response) error {
	if conn, ok := w.(net.Conn); ok {
		_ = conn.SetWriteDeadline(time.Now().Add(workerWriteTimeout))
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(resp)
}

func writeResponse(w io.Writer, resp pluginipc.Response) {
	if err := encodeResponse(w, resp); err != nil {
		log.Printf("jsworker write response failed: %v", err)
	}
}

func handleHealthRequest(req pluginipc.Request, opts workerOptions) pluginipc.Response {
	resp := pluginipc.Response{
		Success: true,
		Healthy: true,
		Version: workerVersion,
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
		},
	}

	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		return resp
	}
	fsCtx, fsCtxErr := buildPluginFSRuntimeContext(opts, req.PluginID, req.PluginName, scriptPath)
	if fsCtxErr != nil {
		resp.Success = false
		resp.Healthy = false
		resp.Error = fsCtxErr.Error()
		return resp
	}

	effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
	result, err := runScriptFunction(
		scriptPath,
		"health",
		[]interface{}{req.PluginConfig, req.Sandbox},
		effectiveOpts,
		req.Sandbox,
		req.HostAPI,
		reqTimeout(req, opts),
		req.PluginSecrets,
		req.Webhook,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		if errors.Is(err, errFunctionNotFound) {
			return resp
		}
		resp.Success = false
		resp.Healthy = false
		resp.Error = err.Error()
		return resp
	}

	exported := exportGojaValue(result)
	if healthMap, ok := exported.(map[string]interface{}); ok {
		if healthy, ok := interfaceToBool(healthMap["healthy"]); ok {
			resp.Healthy = healthy
			resp.Success = healthy
		}
		if version, ok := healthMap["version"].(string); ok && strings.TrimSpace(version) != "" {
			resp.Version = version
		}
		if metadataRaw, ok := healthMap["metadata"].(map[string]interface{}); ok {
			resp.Metadata = convertMetadata(metadataRaw, resp.Metadata)
		}
		return resp
	}

	if healthy, ok := interfaceToBool(exported); ok {
		resp.Healthy = healthy
		resp.Success = healthy
		if !healthy {
			resp.Error = "plugin health function returned false"
		}
	}
	return resp
}

func handleRuntimeStateRequest(req pluginipc.Request) pluginipc.Response {
	if req.PluginID == 0 {
		return pluginipc.Response{
			Success: false,
			Error:   "plugin_id is required for runtime_state",
		}
	}
	state := globalPersistentPluginRuntimeManager.snapshot(req.PluginID, req.PluginGeneration)
	if scriptPath := normalizeScriptPath(req.ScriptPath); scriptPath != "" && !state.Exists {
		state.ScriptPath = scriptPath
	}
	return pluginipc.Response{
		Success: true,
		Healthy: true,
		Version: workerVersion,
		Data:    runtimeStateToMap(state),
	}
}

type pluginExecutionState struct {
	mu                  sync.Mutex
	inflight            int
	memoryBreachStreak  int
	memoryBreakerOpened time.Time
}

var pluginExecutionStates sync.Map

func pluginExecutionStateForRequest(req pluginipc.Request) *pluginExecutionState {
	key := pluginExecutionKey(req)
	if existing, ok := pluginExecutionStates.Load(key); ok {
		if state, ok := existing.(*pluginExecutionState); ok {
			return state
		}
	}
	state := &pluginExecutionState{}
	actual, _ := pluginExecutionStates.LoadOrStore(key, state)
	resolved, _ := actual.(*pluginExecutionState)
	if resolved == nil {
		return state
	}
	return resolved
}

func pluginExecutionKey(req pluginipc.Request) string {
	if req.PluginID > 0 {
		return fmt.Sprintf("id:%d:gen:%d", req.PluginID, normalizePersistentPluginGeneration(req.PluginGeneration))
	}
	name := strings.TrimSpace(req.PluginName)
	if name != "" {
		return "name:" + name
	}
	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath != "" {
		return "script:" + scriptPath
	}
	return "anonymous"
}

func (s *pluginExecutionState) acquire(maxConcurrency int) error {
	if s == nil {
		return nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memoryBreakerOpened.After(now) {
		retryAfter := s.memoryBreakerOpened.Sub(now).Round(time.Second)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return fmt.Errorf("plugin execution blocked by memory circuit breaker, retry after %s", retryAfter)
	}
	if s.inflight >= maxConcurrency {
		return fmt.Errorf("plugin concurrency limit exceeded (limit=%d)", maxConcurrency)
	}
	s.inflight++
	return nil
}

func (s *pluginExecutionState) release() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.inflight > 0 {
		s.inflight--
	}
	s.mu.Unlock()
}

func (s *pluginExecutionState) recordResult(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil && errors.Is(err, errExecutionMemoryLimitExceeded) {
		s.memoryBreachStreak++
		if s.memoryBreachStreak >= memoryBreakerThreshold {
			s.memoryBreakerOpened = time.Now().Add(memoryBreakerCooldown)
			s.memoryBreachStreak = 0
		}
		return
	}
	s.memoryBreachStreak = 0
}

func handleExecuteRequest(conn net.Conn, req pluginipc.Request, opts workerOptions) pluginipc.Response {
	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		return pluginipc.Response{
			Success: false,
			Error:   "script_path is required for execute",
		}
	}

	ctx := map[string]interface{}{}
	if req.Context != nil {
		ctx["user_id"] = req.Context.UserID
		ctx["order_id"] = req.Context.OrderID
		ctx["session_id"] = req.Context.SessionID
		ctx["metadata"] = req.Context.Metadata
	}

	if req.PluginID > 0 {
		effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
		state := pluginExecutionStateForRequest(req)
		if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
			return pluginipc.Response{
				Success: false,
				Error:   err.Error(),
			}
		}
		defer state.release()

		runtime, releaseRuntime, err := globalPersistentPluginRuntimeManager.acquire(req, opts)
		if err != nil {
			state.recordResult(err)
			return pluginipc.Response{
				Success: false,
				Error:   err.Error(),
			}
		}
		defer releaseRuntime()

		execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
		defer stopExecCtx()
		workspaceState := newPluginWorkspaceState(req.Workspace)
		workspaceState.configureCommand(req.Action, req.Params)

		result, storageSnapshot, storageChanged, storageAccessMode, err := runtime.execute(
			"execute",
			[]interface{}{req.Action, req.Params, ctx, req.PluginConfig, req.Sandbox},
			req.Sandbox,
			req.HostAPI,
			reqTimeout(req, opts),
			req.Context,
			req.PluginConfig,
			req.Storage,
			req.PluginSecrets,
			req.Webhook,
			execCtx,
			workspaceState,
		)
		state.recordResult(err)
		metadata := buildJSWorkerExecutionMetadata(map[string]string{"runtime": "goja"}, storageAccessMode)
		if err != nil {
			return buildExecutionErrorResponse(err, storageSnapshot, storageChanged, metadata, true, workspaceState)
		}
		return buildExecutionResponse(result, storageSnapshot, storageChanged, metadata, true, workspaceState)
	}
	fsCtx, fsCtxErr := buildPluginFSRuntimeContext(opts, req.PluginID, req.PluginName, scriptPath)
	if fsCtxErr != nil {
		return pluginipc.Response{
			Success: false,
			Error:   fsCtxErr.Error(),
		}
	}

	effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
	state := pluginExecutionStateForRequest(req)
	if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
		return pluginipc.Response{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer state.release()

	execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
	defer stopExecCtx()
	workspaceState := newPluginWorkspaceState(req.Workspace)
	workspaceState.configureCommand(req.Action, req.Params)

	result, storageSnapshot, storageChanged, storageAccessMode, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{req.Action, req.Params, ctx, req.PluginConfig, req.Sandbox},
		effectiveOpts,
		req.Sandbox,
		req.HostAPI,
		reqTimeout(req, opts),
		req.Storage,
		req.PluginSecrets,
		req.Webhook,
		execCtx,
		fsCtx,
		workspaceState,
	)
	state.recordResult(err)
	metadata := buildJSWorkerExecutionMetadata(map[string]string{"runtime": "goja"}, storageAccessMode)
	if err != nil {
		return buildExecutionErrorResponse(err, storageSnapshot, storageChanged, metadata, true, workspaceState)
	}
	return buildExecutionResponse(result, storageSnapshot, storageChanged, metadata, true, workspaceState)
}

func handleRuntimeEvalRequest(conn net.Conn, req pluginipc.Request, opts workerOptions) pluginipc.Response {
	if req.PluginID == 0 {
		return pluginipc.Response{
			Success: false,
			Error:   "plugin_id is required for runtime_eval",
		}
	}
	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		return pluginipc.Response{
			Success: false,
			Error:   "script_path is required for runtime_eval",
		}
	}
	if strings.TrimSpace(req.RuntimeCode) == "" {
		return pluginipc.Response{
			Success: false,
			Error:   "runtime_code is required for runtime_eval",
		}
	}

	effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
	state := pluginExecutionStateForRequest(req)
	if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
		return pluginipc.Response{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer state.release()

	runtime, releaseRuntime, err := globalPersistentPluginRuntimeManager.acquire(req, opts)
	if err != nil {
		state.recordResult(err)
		return pluginipc.Response{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer releaseRuntime()

	execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
	defer stopExecCtx()
	workspaceState := newPluginWorkspaceState(req.Workspace)
	workspaceState.configureRuntimeConsole(req.Action, req.Context)

	preview, storageSnapshot, storageChanged, storageAccessMode, err := runtime.evaluateRuntime(
		req.RuntimeCode,
		req.Sandbox,
		req.HostAPI,
		reqTimeout(req, opts),
		req.Context,
		req.PluginConfig,
		req.Storage,
		req.PluginSecrets,
		req.Webhook,
		execCtx,
		workspaceState,
	)
	state.recordResult(err)
	metadata := buildJSWorkerExecutionMetadata(map[string]string{
		"runtime":                "goja",
		"workspace_console_mode": "eval",
	}, storageAccessMode)
	if err != nil {
		return buildExecutionErrorResponse(err, storageSnapshot, storageChanged, metadata, true, workspaceState)
	}
	data := runtimeConsolePreviewToMap(preview)
	data["runtime_state"] = runtimeStateToMap(runtime.snapshotState())
	workspaceEntries, workspaceCleared := workspaceState.flushDelta()
	return pluginipc.Response{
		Success:          true,
		Healthy:          true,
		Version:          workerVersion,
		Data:             data,
		Storage:          storageSnapshot,
		StorageChanged:   storageChanged,
		Metadata:         metadata,
		WorkspaceEntries: workspaceEntries,
		WorkspaceCleared: workspaceCleared,
		IsFinal:          true,
	}
}

func handleRuntimeInspectRequest(conn net.Conn, req pluginipc.Request, opts workerOptions) pluginipc.Response {
	if req.PluginID == 0 {
		return pluginipc.Response{
			Success: false,
			Error:   "plugin_id is required for runtime_inspect",
		}
	}
	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		return pluginipc.Response{
			Success: false,
			Error:   "script_path is required for runtime_inspect",
		}
	}

	effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
	state := pluginExecutionStateForRequest(req)
	if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
		return pluginipc.Response{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer state.release()

	runtime, releaseRuntime, err := globalPersistentPluginRuntimeManager.acquire(req, opts)
	if err != nil {
		state.recordResult(err)
		return pluginipc.Response{
			Success: false,
			Error:   err.Error(),
		}
	}
	defer releaseRuntime()

	execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
	defer stopExecCtx()
	workspaceState := newPluginWorkspaceState(req.Workspace)
	workspaceState.configureRuntimeConsole(req.Action, req.Context)

	preview, storageSnapshot, storageChanged, storageAccessMode, err := runtime.inspectRuntime(
		req.RuntimeInspectExpression,
		req.RuntimeInspectDepth,
		req.Sandbox,
		req.HostAPI,
		reqTimeout(req, opts),
		req.Context,
		req.PluginConfig,
		req.Storage,
		req.PluginSecrets,
		req.Webhook,
		execCtx,
		workspaceState,
	)
	state.recordResult(err)
	metadata := buildJSWorkerExecutionMetadata(map[string]string{
		"runtime":                "goja",
		"workspace_console_mode": "inspect",
	}, storageAccessMode)
	if err != nil {
		return buildExecutionErrorResponse(err, storageSnapshot, storageChanged, metadata, true, workspaceState)
	}
	data := runtimeConsolePreviewToMap(preview)
	data["runtime_state"] = runtimeStateToMap(runtime.snapshotState())
	workspaceEntries, workspaceCleared := workspaceState.flushDelta()
	return pluginipc.Response{
		Success:          true,
		Healthy:          true,
		Version:          workerVersion,
		Data:             data,
		Storage:          storageSnapshot,
		StorageChanged:   storageChanged,
		Metadata:         metadata,
		WorkspaceEntries: workspaceEntries,
		WorkspaceCleared: workspaceCleared,
		IsFinal:          true,
	}
}

func handleExecuteStreamRequest(conn net.Conn, req pluginipc.Request, opts workerOptions) {
	streamMetadata := map[string]string{
		"runtime": "goja",
		"stream":  "true",
	}

	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		writeResponse(conn, pluginipc.Response{
			Success:  false,
			Error:    "script_path is required for execute_stream",
			Metadata: streamMetadata,
			IsFinal:  true,
		})
		return
	}

	if req.PluginID > 0 {
		effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
		state := pluginExecutionStateForRequest(req)
		if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
			writeResponse(conn, pluginipc.Response{
				Success:  false,
				Error:    err.Error(),
				Metadata: streamMetadata,
				IsFinal:  true,
			})
			return
		}
		defer state.release()

		runtime, releaseRuntime, err := globalPersistentPluginRuntimeManager.acquire(req, opts)
		if err != nil {
			state.recordResult(err)
			writeResponse(conn, pluginipc.Response{
				Success:  false,
				Error:    err.Error(),
				Metadata: streamMetadata,
				IsFinal:  true,
			})
			return
		}
		defer releaseRuntime()

		encoder := json.NewEncoder(conn)
		workspaceState := newPluginWorkspaceState(req.Workspace)
		workspaceState.configureCommand(req.Action, req.Params)
		emitChunk := func(data map[string]interface{}, metadata map[string]string) error {
			_ = conn.SetWriteDeadline(time.Now().Add(workerWriteTimeout))
			workspaceEntries, workspaceCleared := workspaceState.flushDelta()
			return encoder.Encode(pluginipc.Response{
				Success:          true,
				Healthy:          true,
				Version:          workerVersion,
				Data:             data,
				Metadata:         mergeStringMaps(streamMetadata, metadata),
				WorkspaceEntries: workspaceEntries,
				WorkspaceCleared: workspaceCleared,
				IsFinal:          false,
			})
		}

		execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
		defer stopExecCtx()

		ctx := map[string]interface{}{}
		if req.Context != nil {
			ctx["user_id"] = req.Context.UserID
			ctx["order_id"] = req.Context.OrderID
			ctx["session_id"] = req.Context.SessionID
			ctx["metadata"] = req.Context.Metadata
		}

		result, storageSnapshot, storageChanged, storageAccessMode, err := runtime.executeStream(
			[]interface{}{req.Action, req.Params, ctx, req.PluginConfig, req.Sandbox},
			req.Sandbox,
			req.HostAPI,
			reqTimeout(req, opts),
			req.Context,
			req.PluginConfig,
			req.Storage,
			req.PluginSecrets,
			req.Webhook,
			execCtx,
			workspaceState,
			emitChunk,
		)
		state.recordResult(err)
		finalMetadata := buildJSWorkerExecutionMetadata(streamMetadata, storageAccessMode)
		if err != nil {
			writeResponse(conn, buildExecutionErrorResponse(err, storageSnapshot, storageChanged, finalMetadata, true, workspaceState))
			return
		}
		writeResponse(conn, buildExecutionResponse(result, storageSnapshot, storageChanged, finalMetadata, true, workspaceState))
		return
	}

	ctx := map[string]interface{}{}
	if req.Context != nil {
		ctx["user_id"] = req.Context.UserID
		ctx["order_id"] = req.Context.OrderID
		ctx["session_id"] = req.Context.SessionID
		ctx["metadata"] = req.Context.Metadata
	}
	fsCtx, fsCtxErr := buildPluginFSRuntimeContext(opts, req.PluginID, req.PluginName, scriptPath)
	if fsCtxErr != nil {
		writeResponse(conn, pluginipc.Response{
			Success:  false,
			Error:    fsCtxErr.Error(),
			Metadata: streamMetadata,
			IsFinal:  true,
		})
		return
	}

	effectiveOpts := applyRequestSandbox(opts, req.Sandbox)
	state := pluginExecutionStateForRequest(req)
	if err := state.acquire(effectiveOpts.maxConcurrency); err != nil {
		writeResponse(conn, pluginipc.Response{
			Success:  false,
			Error:    err.Error(),
			Metadata: streamMetadata,
			IsFinal:  true,
		})
		return
	}
	defer state.release()

	encoder := json.NewEncoder(conn)
	workspaceState := newPluginWorkspaceState(req.Workspace)
	workspaceState.configureCommand(req.Action, req.Params)
	emitChunk := func(data map[string]interface{}, metadata map[string]string) error {
		_ = conn.SetWriteDeadline(time.Now().Add(workerWriteTimeout))
		workspaceEntries, workspaceCleared := workspaceState.flushDelta()
		return encoder.Encode(pluginipc.Response{
			Success:          true,
			Healthy:          true,
			Version:          workerVersion,
			Data:             data,
			Metadata:         mergeStringMaps(streamMetadata, metadata),
			WorkspaceEntries: workspaceEntries,
			WorkspaceCleared: workspaceCleared,
			IsFinal:          false,
		})
	}

	execCtx, stopExecCtx := startWorkerConnectionExecutionContext(conn)
	defer stopExecCtx()

	result, storageSnapshot, storageChanged, storageAccessMode, err := runStreamScriptFunctionWithStorage(
		scriptPath,
		[]interface{}{req.Action, req.Params, ctx, req.PluginConfig, req.Sandbox},
		effectiveOpts,
		req.Sandbox,
		req.HostAPI,
		reqTimeout(req, opts),
		req.Storage,
		req.PluginSecrets,
		req.Webhook,
		execCtx,
		fsCtx,
		workspaceState,
		emitChunk,
	)
	state.recordResult(err)
	finalMetadata := buildJSWorkerExecutionMetadata(streamMetadata, storageAccessMode)
	if err != nil {
		writeResponse(conn, buildExecutionErrorResponse(err, storageSnapshot, storageChanged, finalMetadata, true, workspaceState))
		return
	}

	writeResponse(conn, buildExecutionResponse(result, storageSnapshot, storageChanged, finalMetadata, true, workspaceState))
}

func startWorkerConnectionExecutionContext(conn net.Conn) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	if conn == nil {
		return ctx, cancel
	}

	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := conn.Read(buf); err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					if ctx.Err() != nil {
						return
					}
					continue
				}
				cancel()
				return
			}
			cancel()
			return
		}
	}()

	return ctx, cancel
}

var errFunctionNotFound = errors.New("function not found")
var errExecutionMemoryLimitExceeded = errors.New("execution memory limit exceeded")

func runScriptFunction(
	scriptPath string,
	functionName string,
	args []interface{},
	opts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	fsCtx pluginFSRuntimeContext,
) (goja.Value, error) {
	value, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		functionName,
		args,
		opts,
		sandboxCfg,
		hostCfg,
		timeout,
		nil,
		secretSnapshot,
		webhookReq,
		execCtx,
		fsCtx,
		nil,
	)
	return value, err
}

func runScriptFunctionWithStorage(
	scriptPath string,
	functionName string,
	args []interface{},
	opts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	fsCtx pluginFSRuntimeContext,
	workspaceState ...*pluginWorkspaceState,
) (goja.Value, map[string]string, bool, string, error) {
	var resolvedWorkspaceState *pluginWorkspaceState
	if len(workspaceState) > 0 {
		resolvedWorkspaceState = workspaceState[0]
	}
	return runScriptWithStorage(
		scriptPath,
		opts,
		sandboxCfg,
		hostCfg,
		timeout,
		storageSnapshot,
		secretSnapshot,
		webhookReq,
		execCtx,
		fsCtx,
		resolvedWorkspaceState,
		func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error) {
			fn, thisArg, resolveErr := resolveScriptFunction(vm, entryModule, functionName)
			if resolveErr != nil {
				return nil, resolveErr
			}
			if fn == nil {
				return nil, errFunctionNotFound
			}
			return callScriptFunction(vm, fn, thisArg, functionName, args)
		},
	)
}

func runStreamScriptFunctionWithStorage(
	scriptPath string,
	args []interface{},
	opts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	fsCtx pluginFSRuntimeContext,
	workspaceState *pluginWorkspaceState,
	emit func(map[string]interface{}, map[string]string) error,
) (goja.Value, map[string]string, bool, string, error) {
	return runScriptWithStorage(
		scriptPath,
		opts,
		sandboxCfg,
		hostCfg,
		timeout,
		storageSnapshot,
		secretSnapshot,
		webhookReq,
		execCtx,
		fsCtx,
		workspaceState,
		func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error) {
			streamWriter := newPluginStreamWriter(vm, emit)

			fn, thisArg, resolveErr := resolveScriptFunction(vm, entryModule, "executeStream")
			if resolveErr == nil && fn != nil {
				return callScriptFunction(vm, fn, thisArg, "executeStream", append(args, streamWriter))
			}
			if resolveErr != nil && !errors.Is(resolveErr, errFunctionNotFound) {
				return nil, resolveErr
			}

			fn, thisArg, resolveErr = resolveScriptFunction(vm, entryModule, "execute")
			if resolveErr != nil {
				return nil, resolveErr
			}
			if fn == nil {
				return nil, errFunctionNotFound
			}
			return callScriptFunction(vm, fn, thisArg, "execute", append(args, streamWriter))
		},
	)
}

func runScriptWithStorage(
	scriptPath string,
	opts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	fsCtx pluginFSRuntimeContext,
	workspaceState *pluginWorkspaceState,
	invoke func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error),
) (goja.Value, map[string]string, bool, string, error) {
	scriptPath = normalizeScriptPath(scriptPath)
	if scriptPath == "" {
		return nil, nil, false, storageAccessNone, fmt.Errorf("script path is required")
	}
	if !fileExists(scriptPath) {
		return nil, nil, false, storageAccessNone, fmt.Errorf("script not found: %s", scriptPath)
	}
	if execCtx == nil {
		execCtx = context.Background()
	}
	if timeout <= 0 {
		timeout = time.Duration(opts.timeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	releaseHostSession := attachPluginHostSession(hostCfg)
	defer releaseHostSession()

	vm := goja.New()
	storageState := newPluginStorageState(storageSnapshot, pluginStorageLimitsFromOptions(opts))
	asyncState := newSingleUseRuntimeAsyncState(vm, execCtx, time.Now().UTC().Add(timeout))
	moduleRoot := fsCtx.CodeRoot
	if strings.TrimSpace(moduleRoot) == "" {
		moduleRoot = detectModuleRoot(scriptPath)
		fsCtx.CodeRoot = moduleRoot
	}
	registerGlobals(vm, opts, sandboxCfg, hostCfg, storageState, secretSnapshot, webhookReq, fsCtx, workspaceState, asyncState)
	moduleLoader := newCommonJSLoader(vm, scriptPath)

	entryDir := filepath.Dir(scriptPath)
	entryModule := vm.NewObject()
	entryExports := vm.NewObject()
	_ = entryModule.Set("exports", entryExports)

	_ = vm.Set("require", moduleLoader.makeRequire(entryDir))
	_ = vm.Set("module", entryModule)
	_ = vm.Set("exports", entryExports)
	_ = vm.Set("__filename", filepath.ToSlash(scriptPath))
	_ = vm.Set("__dirname", filepath.ToSlash(entryDir))

	resultCh := make(chan struct {
		value goja.Value
		err   error
	}, 1)
	var memoryExceeded atomic.Bool
	stopMemoryMonitor := func() {}
	if opts.maxMemoryMB > 0 {
		limitBytes := uint64(opts.maxMemoryMB) * 1024 * 1024
		stopMemoryMonitor = startExecutionMemoryMonitor(limitBytes, memoryMonitorInterval, readRuntimeHeapAlloc, func(_ uint64) {
			memoryExceeded.Store(true)
			vm.Interrupt("execution memory limit exceeded")
		})
	}

	go func() {
		var out struct {
			value goja.Value
			err   error
		}
		defer func() {
			if asyncState != nil {
				asyncState.finish()
			}
			if r := recover(); r != nil {
				out.err = fmt.Errorf("panic in script runtime: %v", r)
			}
			resultCh <- out
		}()

		program, compileErr := loadWorkerProgram(scriptPath, workerProgramWrapperNone)
		if compileErr != nil {
			out.err = compileErr
			return
		}
		if _, runErr := vm.RunProgram(program); runErr != nil {
			out.err = fmt.Errorf("run script failed: %w", runErr)
			return
		}

		value, invokeErr := invoke(vm, entryModule)
		if invokeErr != nil {
			out.err = invokeErr
			return
		}
		value, invokeErr = asyncState.resolveAsyncResult(value)
		if invokeErr != nil {
			out.err = invokeErr
			return
		}
		out.value = value
	}()

	timer := time.AfterFunc(timeout, func() {
		vm.Interrupt("execution timeout")
	})
	defer timer.Stop()
	stopContextInterrupt := context.AfterFunc(execCtx, func() {
		if err := execCtx.Err(); err != nil {
			vm.Interrupt(err)
		}
	})
	defer stopContextInterrupt()

	result := <-resultCh
	stopMemoryMonitor()
	if memoryExceeded.Load() {
		return nil, storageState.snapshot(), storageState.changed, storageState.accessMode(), fmt.Errorf("%w (limit=%dMB)", errExecutionMemoryLimitExceeded, opts.maxMemoryMB)
	}
	if result.err != nil {
		return nil, storageState.snapshot(), storageState.changed, storageState.accessMode(), result.err
	}
	return result.value, storageState.snapshot(), storageState.changed, storageState.accessMode(), nil
}

func callScriptFunction(
	vm *goja.Runtime,
	fn goja.Callable,
	thisArg goja.Value,
	functionName string,
	args []interface{},
) (goja.Value, error) {
	callArgs := make([]goja.Value, 0, len(args))
	for _, item := range args {
		switch typed := item.(type) {
		case goja.Value:
			callArgs = append(callArgs, typed)
		case pluginipc.SandboxConfig:
			callArgs = append(callArgs, vm.Get("sandbox"))
		default:
			callArgs = append(callArgs, vm.ToValue(item))
		}
	}
	value, callErr := fn(thisArg, callArgs...)
	if callErr != nil {
		return nil, fmt.Errorf("call %s failed: %w", functionName, callErr)
	}
	return value, nil
}

func readRuntimeHeapAlloc() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

func startExecutionMemoryMonitor(limitBytes uint64, interval time.Duration, readHeap func() uint64, onLimitExceeded func(growth uint64)) func() {
	if limitBytes == 0 || readHeap == nil || onLimitExceeded == nil {
		return func() {}
	}
	interval = normalizeExecutionMemoryMonitorInterval(interval)

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	baseline := readHeap()

	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				current := readHeap()
				if current <= baseline {
					continue
				}
				growth := current - baseline
				if growth > limitBytes {
					onLimitExceeded(growth)
					return
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(stopCh)
			<-doneCh
		})
	}
}

func normalizeExecutionMemoryMonitorInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = memoryMonitorInterval
	}
	if interval < memoryMonitorMinInterval {
		return memoryMonitorMinInterval
	}
	if interval > memoryMonitorMaxInterval {
		return memoryMonitorMaxInterval
	}
	return interval
}

type commonJSModule struct {
	exports goja.Value
}

type commonJSLoader struct {
	vm                     *goja.Runtime
	rootDir                string
	cache                  map[string]*commonJSModule
	resolveCache           map[string]string
	packageEntrypointCache map[string][]string
}

func newCommonJSLoader(vm *goja.Runtime, entryScriptPath string) *commonJSLoader {
	return &commonJSLoader{
		vm:                     vm,
		rootDir:                detectModuleRoot(entryScriptPath),
		cache:                  make(map[string]*commonJSModule),
		resolveCache:           make(map[string]string),
		packageEntrypointCache: make(map[string][]string),
	}
}

func (l *commonJSLoader) makeRequire(baseDir string) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		moduleID := ""
		if len(call.Arguments) > 0 {
			moduleID = call.Arguments[0].String()
		}
		value, err := l.require(baseDir, moduleID)
		if err != nil {
			panic(l.vm.NewGoError(err))
		}
		return value
	}
}

func (l *commonJSLoader) require(baseDir string, moduleID string) (goja.Value, error) {
	resolvedPath, err := l.resolveModulePath(baseDir, moduleID)
	if err != nil {
		return nil, err
	}

	if cached, exists := l.cache[resolvedPath]; exists {
		return cached.exports, nil
	}

	exportsObj := l.vm.NewObject()
	moduleObj := l.vm.NewObject()
	_ = moduleObj.Set("exports", exportsObj)
	l.cache[resolvedPath] = &commonJSModule{exports: exportsObj}

	program, err := loadWorkerProgram(resolvedPath, workerProgramWrapperCommonJS)
	if err != nil {
		return nil, fmt.Errorf("prepare module %s failed: %w", moduleID, err)
	}

	wrappedValue, err := l.vm.RunProgram(program)
	if err != nil {
		return nil, fmt.Errorf("run module %s failed: %w", moduleID, err)
	}

	wrappedFn, ok := goja.AssertFunction(wrappedValue)
	if !ok {
		return nil, fmt.Errorf("invalid module wrapper for %s", moduleID)
	}

	moduleDir := filepath.Dir(resolvedPath)
	_, err = wrappedFn(
		goja.Undefined(),
		exportsObj,
		moduleObj,
		l.vm.ToValue(l.makeRequire(moduleDir)),
		l.vm.ToValue(filepath.ToSlash(resolvedPath)),
		l.vm.ToValue(filepath.ToSlash(moduleDir)),
	)
	if err != nil {
		return nil, fmt.Errorf("execute module %s failed: %w", moduleID, err)
	}

	moduleExports := moduleObj.Get("exports")
	if moduleExports == nil || goja.IsUndefined(moduleExports) || goja.IsNull(moduleExports) {
		moduleExports = l.vm.NewObject()
	}
	l.cache[resolvedPath].exports = moduleExports
	return moduleExports, nil
}

func (l *commonJSLoader) resolveModulePath(baseDir string, moduleID string) (string, error) {
	trimmed := strings.TrimSpace(moduleID)
	if trimmed == "" {
		return "", fmt.Errorf("module path is required")
	}

	normalized := filepath.FromSlash(trimmed)
	if baseDir == "" {
		baseDir = l.rootDir
	}
	cacheKey := filepath.ToSlash(filepath.Clean(baseDir)) + "|" + trimmed
	if l != nil && l.resolveCache != nil {
		if resolvedPath, ok := l.resolveCache[cacheKey]; ok && strings.TrimSpace(resolvedPath) != "" {
			return resolvedPath, nil
		}
	}

	var candidateBase string
	if filepath.IsAbs(normalized) {
		candidateBase = filepath.Clean(normalized)
	} else {
		if !strings.HasPrefix(normalized, ".") {
			resolvedPath, err := l.resolvePackageModulePath(trimmed)
			if err == nil && l != nil && l.resolveCache != nil {
				l.resolveCache[cacheKey] = resolvedPath
			}
			return resolvedPath, err
		}
		candidateBase = filepath.Clean(filepath.Join(baseDir, normalized))
	}

	if resolvedPath, ok := resolveCommonJSCandidatePath(l.rootDir, candidateBase); ok {
		if l != nil && l.resolveCache != nil {
			l.resolveCache[cacheKey] = resolvedPath
		}
		return resolvedPath, nil
	}
	return "", fmt.Errorf("module not found: %s", moduleID)
}

func (l *commonJSLoader) resolvePackageModulePath(moduleID string) (string, error) {
	packageName, subPath, err := splitPackageModuleID(moduleID)
	if err != nil {
		return "", err
	}

	packageDir := filepath.Clean(filepath.Join(l.rootDir, "node_modules", filepath.FromSlash(packageName)))
	if !isPathWithinRoot(l.rootDir, packageDir) {
		return "", fmt.Errorf("module not found: %s", moduleID)
	}
	packageDir, err = resolvePathWithinRoot(l.rootDir, packageDir)
	if err != nil {
		return "", fmt.Errorf("module not found: %s", moduleID)
	}

	if strings.TrimSpace(subPath) != "" {
		candidateBase := filepath.Clean(filepath.Join(packageDir, filepath.FromSlash(subPath)))
		if resolvedPath, ok := resolveCommonJSCandidatePath(l.rootDir, candidateBase); ok {
			return resolvedPath, nil
		}
		return "", fmt.Errorf("module not found: %s", moduleID)
	}

	for _, candidateBase := range l.resolvePackageEntrypointCandidates(packageDir) {
		if resolvedPath, ok := resolveCommonJSCandidatePath(l.rootDir, candidateBase); ok {
			return resolvedPath, nil
		}
	}

	return "", fmt.Errorf("module not found: %s", moduleID)
}

func (l *commonJSLoader) resolvePackageEntrypointCandidates(packageDir string) []string {
	normalizedDir := filepath.Clean(packageDir)
	if l != nil && l.packageEntrypointCache != nil {
		if cached, ok := l.packageEntrypointCache[normalizedDir]; ok {
			return append([]string(nil), cached...)
		}
	}
	candidates := resolvePackageEntrypointCandidates(normalizedDir)
	if l != nil && l.packageEntrypointCache != nil {
		l.packageEntrypointCache[normalizedDir] = append([]string(nil), candidates...)
	}
	return candidates
}

func resolveCommonJSCandidatePath(rootDir string, candidateBase string) (string, bool) {
	candidates := make([]string, 0, 3)
	candidates = append(candidates, candidateBase)
	if filepath.Ext(candidateBase) == "" {
		candidates = append(candidates, candidateBase+".js")
		candidates = append(candidates, filepath.Join(candidateBase, "index.js"))
	}

	for _, candidate := range candidates {
		cleanPath := filepath.Clean(candidate)
		resolvedPath, err := resolvePathWithinRoot(rootDir, cleanPath)
		if err != nil {
			continue
		}
		if fileExists(resolvedPath) {
			return resolvedPath, true
		}
	}
	return "", false
}

func splitPackageModuleID(moduleID string) (string, string, error) {
	trimmed := strings.TrimSpace(moduleID)
	if trimmed == "" {
		return "", "", fmt.Errorf("module path is required")
	}

	normalized := filepath.ToSlash(trimmed)
	if strings.HasPrefix(normalized, ".") || strings.HasPrefix(normalized, "/") {
		return "", "", fmt.Errorf("invalid package module path: %s", moduleID)
	}

	parts := strings.Split(normalized, "/")
	if len(parts) == 0 {
		return "", "", fmt.Errorf("invalid package module path: %s", moduleID)
	}

	if strings.HasPrefix(parts[0], "@") {
		if len(parts) < 2 {
			return "", "", fmt.Errorf("invalid scoped package module path: %s", moduleID)
		}
		packageName := parts[0] + "/" + parts[1]
		return packageName, strings.Join(parts[2:], "/"), nil
	}

	return parts[0], strings.Join(parts[1:], "/"), nil
}

func resolvePackageEntrypointCandidates(packageDir string) []string {
	candidates := make([]string, 0, 2)
	if mainPath := readPackageMain(packageDir); mainPath != "" {
		candidates = append(candidates, filepath.Clean(filepath.Join(packageDir, filepath.FromSlash(mainPath))))
	}
	candidates = append(candidates, filepath.Join(packageDir, "index"))
	return candidates
}

func readPackageMain(packageDir string) string {
	type packageManifest struct {
		Main string `json:"main"`
	}

	raw, err := os.ReadFile(filepath.Join(packageDir, "package.json"))
	if err != nil {
		return ""
	}

	var manifest packageManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ""
	}
	return strings.TrimSpace(manifest.Main)
}

func resolveScriptFunction(vm *goja.Runtime, entryModule *goja.Object, functionName string) (goja.Callable, goja.Value, error) {
	trimmedName := strings.TrimSpace(functionName)
	if trimmedName == "" {
		return nil, nil, errFunctionNotFound
	}

	if entryModule != nil {
		moduleExports := entryModule.Get("exports")
		exportsObj := moduleExports.ToObject(vm)
		if exportsObj != nil {
			if fn, ok := goja.AssertFunction(exportsObj.Get(trimmedName)); ok {
				return fn, exportsObj, nil
			}
		}
	}

	if fn, ok := goja.AssertFunction(vm.Get(trimmedName)); ok {
		return fn, goja.Undefined(), nil
	}

	return nil, nil, errFunctionNotFound
}

func detectModuleRoot(entryScriptPath string) string {
	entryDir := filepath.Clean(filepath.Dir(entryScriptPath))
	current := entryDir
	for i := 0; i < 16; i++ {
		if hasManifest(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return entryDir
}

func hasManifest(dir string) bool {
	manifestNames := []string{"manifest.json", "plugin.json", "plugin-manifest.json"}
	for _, name := range manifestNames {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isPathWithinRoot(root string, target string) bool {
	rootClean := filepath.Clean(root)
	targetClean := filepath.Clean(target)
	rel, err := filepath.Rel(rootClean, targetClean)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	parentPrefix := ".." + string(os.PathSeparator)
	return !strings.HasPrefix(rel, parentPrefix)
}

func applyRequestSandbox(opts workerOptions, req pluginipc.SandboxConfig) workerOptions {
	effective := opts
	effective.allowNetwork = opts.allowNetwork && req.AllowNetwork
	effective.allowFS = opts.allowFS && req.AllowFileSystem
	if req.MaxMemoryMB > 0 && (effective.maxMemoryMB <= 0 || req.MaxMemoryMB < effective.maxMemoryMB) {
		effective.maxMemoryMB = req.MaxMemoryMB
	}
	if req.MaxConcurrency > 0 && (effective.maxConcurrency <= 0 || req.MaxConcurrency < effective.maxConcurrency) {
		effective.maxConcurrency = req.MaxConcurrency
	}
	if effective.maxConcurrency <= 0 {
		effective.maxConcurrency = 1
	}
	if req.FSMaxFiles > 0 {
		effective.fsMaxFiles = req.FSMaxFiles
	}
	if req.FSMaxTotalBytes > 0 {
		effective.fsMaxTotalBytes = req.FSMaxTotalBytes
	}
	if req.FSMaxReadBytes > 0 {
		effective.fsMaxReadBytes = req.FSMaxReadBytes
	}
	if effective.fsMaxReadBytes > effective.fsMaxTotalBytes {
		effective.fsMaxReadBytes = effective.fsMaxTotalBytes
	}
	if req.StorageMaxKeys > 0 {
		effective.storageMaxKeys = req.StorageMaxKeys
	}
	if req.StorageMaxTotalBytes > 0 {
		effective.storageMaxTotalBytes = req.StorageMaxTotalBytes
	}
	if req.StorageMaxValueBytes > 0 {
		effective.storageMaxValueBytes = req.StorageMaxValueBytes
	}
	if effective.storageMaxValueBytes > effective.storageMaxTotalBytes {
		effective.storageMaxValueBytes = effective.storageMaxTotalBytes
	}
	return effective
}

type pluginFSRuntimeContext struct {
	CodeRoot   string
	DataRoot   string
	PluginID   uint
	PluginName string
}

func buildPluginFSRuntimeContext(opts workerOptions, pluginID uint, pluginName, scriptPath string) (pluginFSRuntimeContext, error) {
	codeRoot := detectModuleRoot(scriptPath)
	if strings.TrimSpace(codeRoot) == "" {
		return pluginFSRuntimeContext{}, fmt.Errorf("plugin code root is empty")
	}
	codeRoot = filepath.Clean(filepath.FromSlash(codeRoot))
	if !filepath.IsAbs(codeRoot) {
		if absRoot, err := filepath.Abs(codeRoot); err == nil {
			codeRoot = filepath.Clean(absRoot)
		}
	}
	if info, err := os.Stat(codeRoot); err != nil || !info.IsDir() {
		return pluginFSRuntimeContext{}, fmt.Errorf("plugin code root is unavailable: %s", filepath.ToSlash(codeRoot))
	}

	dataLeaf := "plugin_anonymous"
	if pluginID > 0 {
		dataLeaf = fmt.Sprintf("plugin_%d", pluginID)
	} else {
		safeName := sanitizePluginFSIdentifier(pluginName)
		if safeName != "" {
			dataLeaf = "plugin_" + safeName
		}
	}
	dataRoot := filepath.Clean(filepath.Join(opts.artifactRoot, "data", dataLeaf))
	if !filepath.IsAbs(dataRoot) {
		if absRoot, err := filepath.Abs(dataRoot); err == nil {
			dataRoot = filepath.Clean(absRoot)
		}
	}
	return pluginFSRuntimeContext{
		CodeRoot:   codeRoot,
		DataRoot:   dataRoot,
		PluginID:   pluginID,
		PluginName: strings.TrimSpace(pluginName),
	}, nil
}

func sanitizePluginFSIdentifier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, ch := range trimmed {
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if isAlphaNum || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func registerGlobals(
	vm *goja.Runtime,
	opts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	storageState *pluginStorageState,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	fsCtx pluginFSRuntimeContext,
	workspaceState *pluginWorkspaceState,
	asyncState *singleUseRuntimeAsyncState,
) {
	installBrowserCompatibilityPolyfills(vm)
	if asyncState != nil {
		installRuntimeAsyncCompatibilityGlobals(vm, runtimeAsyncGlobalHooks{
			StructuredClone: func(value goja.Value) (goja.Value, error) {
				return runtimeStructuredCloneValue(vm, value)
			},
			QueueMicrotask: func(callback goja.Callable) error {
				return asyncState.queueMicrotask(callback)
			},
			SetTimeout: func(callback goja.Callable, delay time.Duration, args []goja.Value) (int64, error) {
				return asyncState.scheduleTimeout(callback, delay, args)
			},
			ClearTimeout: func(id int64) bool {
				return asyncState.clearTimer(id)
			},
		})
	}

	console := vm.NewObject()
	registerConsoleMethod := func(name string, level string) {
		_ = console.Set(name, func(call goja.FunctionCall) goja.Value {
			if workspaceState != nil && workspaceState.enabled {
				message, metadata := buildRuntimeConsoleCallOutput(vm, call.Arguments)
				log.Printf("[jsworker][plugin][%s] %s", strings.ToUpper(level), message)
				workspaceState.write("console", level, message, "console."+name, metadata)
				return goja.Undefined()
			}
			log.Printf("[jsworker][plugin][%s] %s", strings.ToUpper(level), formatRuntimeConsoleLogOutput(call.Arguments))
			return goja.Undefined()
		})
	}
	registerConsoleMethod("log", "info")
	registerConsoleMethod("info", "info")
	registerConsoleMethod("warn", "warn")
	registerConsoleMethod("error", "error")
	registerConsoleMethod("debug", "debug")
	vm.Set("console", console)

	sandbox := vm.NewObject()
	_ = sandbox.Set("allowNetwork", opts.allowNetwork)
	_ = sandbox.Set("allowFileSystem", opts.allowFS)
	_ = sandbox.Set("currentAction", sandboxCfg.CurrentAction)
	_ = sandbox.Set("declaredStorageAccessMode", sandboxCfg.DeclaredStorageAccess)
	_ = sandbox.Set("storageAccessMode", storageState.accessMode())
	_ = sandbox.Set("allowHookExecute", sandboxCfg.AllowHookExecute)
	_ = sandbox.Set("allowHookBlock", sandboxCfg.AllowHookBlock)
	_ = sandbox.Set("allowPayloadPatch", sandboxCfg.AllowPayloadPatch)
	_ = sandbox.Set("allowFrontendExtensions", sandboxCfg.AllowFrontendExtensions)
	_ = sandbox.Set("allowExecuteAPI", sandboxCfg.AllowExecuteAPI)
	_ = sandbox.Set("requestedPermissions", append([]string{}, sandboxCfg.RequestedPermissions...))
	_ = sandbox.Set("grantedPermissions", append([]string{}, sandboxCfg.GrantedPermissions...))
	_ = sandbox.Set("executeActionStorage", mergeStringMaps(sandboxCfg.ExecuteActionStorage, nil))
	_ = sandbox.Set("defaultTimeoutMs", opts.timeoutMs)
	_ = sandbox.Set("maxConcurrency", opts.maxConcurrency)
	_ = sandbox.Set("maxMemoryMB", opts.maxMemoryMB)
	_ = sandbox.Set("fsMaxFiles", opts.fsMaxFiles)
	_ = sandbox.Set("fsMaxTotalBytes", opts.fsMaxTotalBytes)
	_ = sandbox.Set("fsMaxReadBytes", opts.fsMaxReadBytes)
	_ = sandbox.Set("storageMaxKeys", opts.storageMaxKeys)
	_ = sandbox.Set("storageMaxTotalBytes", opts.storageMaxTotalBytes)
	_ = sandbox.Set("storageMaxValueBytes", opts.storageMaxValueBytes)
	vm.Set("sandbox", sandbox)

	if storageState == nil {
		storageState = newPluginStorageState(nil, pluginStorageLimitsFromOptions(opts))
	}
	pluginFSState, fsInitErr := newPluginFS(fsCtx, opts.fsMaxFiles, opts.fsMaxTotalBytes, opts.fsMaxReadBytes)
	refreshSandboxStorageAccessMode := func() {
		_ = sandbox.Set("storageAccessMode", storageState.accessMode())
	}

	throwJSError := func(err error) {
		if err == nil {
			panic(vm.NewTypeError("unknown plugin fs error"))
		}
		panic(vm.NewGoError(err))
	}
	requireFS := func() *pluginFS {
		if !opts.allowFS {
			panic(vm.NewTypeError("Plugin.fs access denied: allow_file_system=false"))
		}
		if fsInitErr != nil {
			throwJSError(fsInitErr)
		}
		if pluginFSState == nil {
			throwJSError(fmt.Errorf("plugin fs is not available"))
		}
		return pluginFSState
	}
	requireNetwork := func() {
		if !opts.allowNetwork {
			panic(vm.NewTypeError("Plugin.http access denied: allow_network=false"))
		}
	}
	workspaceHostEnabled := workspaceState != nil &&
		isWorkerWorkspaceCommandAction(sandboxCfg.CurrentAction) &&
		hostCfg != nil &&
		strings.TrimSpace(workspaceState.commandID) != ""
	configureWorkerWorkspaceHostBridge(workspaceState, hostCfg, workspaceHostEnabled)
	requestWorkspaceInput := func(prompt string, masked bool, echo bool, source string) (string, bool) {
		value, ok, err := requestWorkerWorkspaceInput(
			workspaceState,
			hostCfg,
			workspaceHostEnabled,
			prompt,
			masked,
			echo,
			source,
		)
		if err != nil {
			throwJSError(err)
		}
		return value, ok
	}

	pluginObj := vm.NewObject()
	workspaceObj := vm.NewObject()
	workspaceEnabled := workspaceState != nil && workspaceState.enabled
	_ = workspaceObj.Set("enabled", workspaceEnabled)
	if workspaceState != nil {
		_ = workspaceObj.Set("commandName", workspaceState.commandName)
		_ = workspaceObj.Set("commandId", workspaceState.commandID)
	} else {
		_ = workspaceObj.Set("commandName", "")
		_ = workspaceObj.Set("commandId", "")
	}
	_ = workspaceObj.Set("write", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || workspaceState == nil {
			return goja.Undefined()
		}
		workspaceState.write(
			"stdout",
			"info",
			call.Arguments[0].String(),
			"plugin.workspace.write",
			workspaceMetadataFromGojaValue(argumentAt(call, 1)),
		)
		return goja.Undefined()
	})
	_ = workspaceObj.Set("writeln", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			return goja.Undefined()
		}
		message := ""
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			message = call.Arguments[0].String()
		}
		workspaceState.write(
			"stdout",
			"info",
			message+"\n",
			"plugin.workspace.writeln",
			workspaceMetadataFromGojaValue(argumentAt(call, 1)),
		)
		return goja.Undefined()
	})
	_ = workspaceObj.Set("info", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || workspaceState == nil {
			return goja.Undefined()
		}
		workspaceState.write("workspace", "info", call.Arguments[0].String(), "plugin.workspace.info", workspaceMetadataFromGojaValue(argumentAt(call, 1)))
		return goja.Undefined()
	})
	_ = workspaceObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || workspaceState == nil {
			return goja.Undefined()
		}
		workspaceState.write("workspace", "warn", call.Arguments[0].String(), "plugin.workspace.warn", workspaceMetadataFromGojaValue(argumentAt(call, 1)))
		return goja.Undefined()
	})
	_ = workspaceObj.Set("error", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 || workspaceState == nil {
			return goja.Undefined()
		}
		workspaceState.write("workspace", "error", call.Arguments[0].String(), "plugin.workspace.error", workspaceMetadataFromGojaValue(argumentAt(call, 1)))
		return goja.Undefined()
	})
	_ = workspaceObj.Set("clear", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			return vm.ToValue(false)
		}
		return vm.ToValue(workspaceState.clear())
	})
	_ = workspaceObj.Set("tail", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			return vm.ToValue([]map[string]interface{}{})
		}
		return vm.ToValue(workspaceEntriesToMaps(workspaceState.tail(workspaceLimitFromGojaArguments(call, 0))))
	})
	_ = workspaceObj.Set("snapshot", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			return vm.ToValue(map[string]interface{}{
				"enabled":     false,
				"max_entries": 0,
				"entry_count": 0,
				"entries":     []map[string]interface{}{},
			})
		}
		return vm.ToValue(workspaceState.snapshot(workspaceLimitFromGojaArguments(call, 0)))
	})
	_ = workspaceObj.Set("read", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			panic(vm.NewTypeError("Plugin.workspace is unavailable"))
		}
		echo, masked := parseWorkspaceReadOptions(argumentAt(call, 0))
		value, ok := requestWorkspaceInput("", masked, echo, "plugin.workspace.read")
		if !ok {
			panic(vm.NewTypeError("Plugin.workspace.read has no input available"))
		}
		return vm.ToValue(value)
	})
	_ = workspaceObj.Set("readLine", func(call goja.FunctionCall) goja.Value {
		if workspaceState == nil {
			panic(vm.NewTypeError("Plugin.workspace is unavailable"))
		}
		prompt := ""
		optionsArgIndex := 0
		if len(call.Arguments) > 0 && call.Arguments[0] != nil && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			if exported := call.Arguments[0].Export(); exported != nil {
				if _, ok := exported.(map[string]interface{}); !ok {
					prompt = call.Arguments[0].String()
					optionsArgIndex = 1
				}
			}
		}
		echo, masked := parseWorkspaceReadOptions(argumentAt(call, optionsArgIndex))
		value, ok := requestWorkspaceInput(prompt, masked, echo, "plugin.workspace.readLine")
		if !ok {
			panic(vm.NewTypeError("Plugin.workspace.readLine has no input available"))
		}
		return vm.ToValue(value)
	})
	_ = pluginObj.Set("workspace", workspaceObj)

	storageObj := vm.NewObject()
	_ = storageObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return goja.Undefined()
		}
		value, ok := storageState.get(key)
		refreshSandboxStorageAccessMode()
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = storageObj.Set("set", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 {
			return vm.ToValue(false)
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		value := call.Arguments[1].String()
		ok := storageState.set(key, value)
		refreshSandboxStorageAccessMode()
		return vm.ToValue(ok)
	})
	_ = storageObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		ok := storageState.delete(key)
		refreshSandboxStorageAccessMode()
		return vm.ToValue(ok)
	})
	_ = storageObj.Set("list", func(call goja.FunctionCall) goja.Value {
		values := storageState.list()
		refreshSandboxStorageAccessMode()
		return vm.ToValue(values)
	})
	_ = storageObj.Set("clear", func(call goja.FunctionCall) goja.Value {
		ok := storageState.clear()
		refreshSandboxStorageAccessMode()
		return vm.ToValue(ok)
	})
	_ = pluginObj.Set("storage", storageObj)

	if secretSnapshot == nil {
		secretSnapshot = map[string]string{}
	}
	secretObj := vm.NewObject()
	_ = secretObj.Set("enabled", true)
	_ = secretObj.Set("get", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return goja.Undefined()
		}
		value, ok := secretSnapshot[key]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = secretObj.Set("has", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return vm.ToValue(false)
		}
		_, ok := secretSnapshot[key]
		return vm.ToValue(ok)
	})
	_ = secretObj.Set("list", func(call goja.FunctionCall) goja.Value {
		keys := make([]string, 0, len(secretSnapshot))
		for key := range secretSnapshot {
			if strings.TrimSpace(key) == "" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return vm.ToValue(keys)
	})
	_ = pluginObj.Set("secret", secretObj)

	webhookEnabled := webhookReq != nil
	webhookKey := ""
	webhookMethod := ""
	webhookPath := ""
	webhookQueryString := ""
	webhookContentType := ""
	webhookRemoteAddr := ""
	webhookBodyText := ""
	webhookBodyBase64 := ""
	webhookHeaders := map[string]string{}
	webhookQueryParams := map[string]string{}
	if webhookReq != nil {
		webhookKey = strings.TrimSpace(webhookReq.Key)
		webhookMethod = strings.TrimSpace(webhookReq.Method)
		webhookPath = strings.TrimSpace(webhookReq.Path)
		webhookQueryString = strings.TrimSpace(webhookReq.QueryString)
		webhookContentType = strings.TrimSpace(webhookReq.ContentType)
		webhookRemoteAddr = strings.TrimSpace(webhookReq.RemoteAddr)
		webhookBodyText = webhookReq.BodyText
		webhookBodyBase64 = strings.TrimSpace(webhookReq.BodyBase64)
		for key, value := range webhookReq.Headers {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if normalizedKey == "" {
				continue
			}
			webhookHeaders[normalizedKey] = value
		}
		for key, value := range webhookReq.QueryParams {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			webhookQueryParams[normalizedKey] = value
		}
	}
	webhookObj := vm.NewObject()
	_ = webhookObj.Set("enabled", webhookEnabled)
	_ = webhookObj.Set("key", webhookKey)
	_ = webhookObj.Set("method", webhookMethod)
	_ = webhookObj.Set("path", webhookPath)
	_ = webhookObj.Set("queryString", webhookQueryString)
	_ = webhookObj.Set("contentType", webhookContentType)
	_ = webhookObj.Set("remoteAddr", webhookRemoteAddr)
	_ = webhookObj.Set("headers", webhookHeaders)
	_ = webhookObj.Set("queryParams", webhookQueryParams)
	_ = webhookObj.Set("bodyText", webhookBodyText)
	_ = webhookObj.Set("bodyBase64", webhookBodyBase64)
	_ = webhookObj.Set("header", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := strings.ToLower(strings.TrimSpace(call.Arguments[0].String()))
		if name == "" {
			return goja.Undefined()
		}
		value, ok := webhookHeaders[name]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = webhookObj.Set("query", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		name := strings.TrimSpace(call.Arguments[0].String())
		if name == "" {
			return goja.Undefined()
		}
		value, ok := webhookQueryParams[name]
		if !ok {
			return goja.Undefined()
		}
		return vm.ToValue(value)
	})
	_ = webhookObj.Set("text", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(webhookBodyText)
	})
	_ = webhookObj.Set("json", func(call goja.FunctionCall) goja.Value {
		if strings.TrimSpace(webhookBodyText) == "" {
			return goja.Undefined()
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(webhookBodyText), &decoded); err != nil {
			throwJSError(fmt.Errorf("Plugin.webhook.json() failed: %w", err))
		}
		return vm.ToValue(decoded)
	})
	_ = pluginObj.Set("webhook", webhookObj)

	httpObj := vm.NewObject()
	_ = httpObj.Set("enabled", opts.allowNetwork)
	_ = httpObj.Set("defaultTimeoutMs", clampPluginHTTPTimeoutMs(0, opts.timeoutMs))
	_ = httpObj.Set("maxResponseBytes", maxPluginHTTPResponseBytes)
	_ = httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		requireNetwork()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.http.get(url, headers?) requires url"))
		}
		headers := map[string]string{}
		if len(call.Arguments) > 1 {
			headers = normalizePluginHTTPHeaders(exportGojaValue(call.Arguments[1]))
		}
		return vm.ToValue(performPluginHTTPRequest(opts, pluginHTTPRequestOptions{
			URL:     call.Arguments[0].String(),
			Method:  http.MethodGet,
			Headers: headers,
		}))
	})
	_ = httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		requireNetwork()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.http.post(url, body?, headers?) requires url"))
		}
		var body interface{}
		headers := map[string]string{}
		if len(call.Arguments) > 1 {
			body = exportGojaValue(call.Arguments[1])
		}
		if len(call.Arguments) > 2 {
			headers = normalizePluginHTTPHeaders(exportGojaValue(call.Arguments[2]))
		}
		return vm.ToValue(performPluginHTTPRequest(opts, pluginHTTPRequestOptions{
			URL:     call.Arguments[0].String(),
			Method:  http.MethodPost,
			Headers: headers,
			Body:    body,
		}))
	})
	_ = httpObj.Set("request", func(call goja.FunctionCall) goja.Value {
		requireNetwork()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.http.request(options) requires options"))
		}
		return vm.ToValue(performPluginHTTPRequest(opts, decodePluginHTTPRequestOptions(exportGojaValue(call.Arguments[0]))))
	})
	_ = pluginObj.Set("http", httpObj)

	hostInvoker := func(action string, params map[string]interface{}) map[string]interface{} {
		result, err := performPluginHostRequest(hostCfg, action, params)
		if err != nil {
			throwJSError(err)
		}
		return result
	}

	hostObj := vm.NewObject()
	_ = hostObj.Set("enabled", hostCfg != nil && strings.TrimSpace(hostCfg.Network) != "" && strings.TrimSpace(hostCfg.Address) != "" && strings.TrimSpace(hostCfg.AccessToken) != "")
	_ = hostObj.Set("invoke", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.host.invoke(action, params?) requires action"))
		}
		action := strings.TrimSpace(call.Arguments[0].String())
		params := map[string]interface{}{}
		if len(call.Arguments) > 1 {
			params = normalizePluginHostParams(exportGojaValue(call.Arguments[1]))
		}
		return vm.ToValue(hostInvoker(action, params))
	})

	orderObj := vm.NewObject()
	_ = orderObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.order.get", params))
	})
	_ = orderObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.order.list", params))
	})
	_ = orderObj.Set("assignTracking", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.order.assign_tracking", params))
	})
	_ = orderObj.Set("requestResubmit", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.order.request_resubmit", params))
	})
	_ = orderObj.Set("markPaid", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.order.mark_paid", params))
	})
	_ = orderObj.Set("updatePrice", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.order.update_price", params))
	})

	userObj := vm.NewObject()
	_ = userObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.user.get", params))
	})
	_ = userObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.user.list", params))
	})

	productObj := vm.NewObject()
	_ = productObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.product.get", params))
	})
	_ = productObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.product.list", params))
	})

	inventoryObj := vm.NewObject()
	_ = inventoryObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.inventory.get", params))
	})
	_ = inventoryObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.inventory.list", params))
	})

	inventoryBindingObj := vm.NewObject()
	_ = inventoryBindingObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.inventory_binding.get", params))
	})
	_ = inventoryBindingObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.inventory_binding.list", params))
	})

	promoObj := vm.NewObject()
	_ = promoObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.promo.get", params))
	})
	_ = promoObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.promo.list", params))
	})

	ticketObj := vm.NewObject()
	_ = ticketObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.ticket.get", params))
	})
	_ = ticketObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.ticket.list", params))
	})
	_ = ticketObj.Set("reply", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.ticket.reply", params))
	})
	_ = ticketObj.Set("update", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.ticket.update", params))
	})

	serialObj := vm.NewObject()
	_ = serialObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.serial.get", params))
	})
	_ = serialObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.serial.list", params))
	})

	announcementObj := vm.NewObject()
	_ = announcementObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.announcement.get", params))
	})
	_ = announcementObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.announcement.list", params))
	})

	knowledgeObj := vm.NewObject()
	_ = knowledgeObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.knowledge.get", params))
	})
	_ = knowledgeObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.knowledge.list", params))
	})
	_ = knowledgeObj.Set("categories", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.knowledge.categories", params))
	})

	paymentMethodObj := vm.NewObject()
	_ = paymentMethodObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.payment_method.get", params))
	})
	_ = paymentMethodObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.payment_method.list", params))
	})

	virtualInventoryObj := vm.NewObject()
	_ = virtualInventoryObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.virtual_inventory.get", params))
	})
	_ = virtualInventoryObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.virtual_inventory.list", params))
	})

	virtualInventoryBindingObj := vm.NewObject()
	_ = virtualInventoryBindingObj.Set("get", func(call goja.FunctionCall) goja.Value {
		params := buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
		return vm.ToValue(hostInvoker("host.virtual_inventory_binding.get", params))
	})
	_ = virtualInventoryBindingObj.Set("list", func(call goja.FunctionCall) goja.Value {
		params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
		return vm.ToValue(hostInvoker("host.virtual_inventory_binding.list", params))
	})

	sharedHostRootObjects := buildSharedPluginHostRootObjects(vm, hostInvoker)
	marketObj := sharedHostRootObjects["market"]
	emailTemplateObj := sharedHostRootObjects["emailTemplate"]
	landingPageObj := sharedHostRootObjects["landingPage"]
	invoiceTemplateObj := sharedHostRootObjects["invoiceTemplate"]
	authBrandingObj := sharedHostRootObjects["authBranding"]
	pageRulePackObj := sharedHostRootObjects["pageRulePack"]

	_ = hostObj.Set("order", orderObj)
	_ = hostObj.Set("user", userObj)
	_ = hostObj.Set("product", productObj)
	_ = hostObj.Set("inventory", inventoryObj)
	_ = hostObj.Set("inventoryBinding", inventoryBindingObj)
	_ = hostObj.Set("promo", promoObj)
	_ = hostObj.Set("ticket", ticketObj)
	_ = hostObj.Set("serial", serialObj)
	_ = hostObj.Set("announcement", announcementObj)
	_ = hostObj.Set("knowledge", knowledgeObj)
	_ = hostObj.Set("paymentMethod", paymentMethodObj)
	_ = hostObj.Set("virtualInventory", virtualInventoryObj)
	_ = hostObj.Set("virtualInventoryBinding", virtualInventoryBindingObj)
	_ = hostObj.Set("market", marketObj)
	_ = hostObj.Set("emailTemplate", emailTemplateObj)
	_ = hostObj.Set("landingPage", landingPageObj)
	_ = hostObj.Set("invoiceTemplate", invoiceTemplateObj)
	_ = hostObj.Set("authBranding", authBrandingObj)
	_ = hostObj.Set("pageRulePack", pageRulePackObj)
	_ = pluginObj.Set("host", hostObj)
	_ = pluginObj.Set("order", orderObj)
	_ = pluginObj.Set("user", userObj)
	_ = pluginObj.Set("product", productObj)
	_ = pluginObj.Set("inventory", inventoryObj)
	_ = pluginObj.Set("inventoryBinding", inventoryBindingObj)
	_ = pluginObj.Set("promo", promoObj)
	_ = pluginObj.Set("ticket", ticketObj)
	_ = pluginObj.Set("serial", serialObj)
	_ = pluginObj.Set("announcement", announcementObj)
	_ = pluginObj.Set("knowledge", knowledgeObj)
	_ = pluginObj.Set("paymentMethod", paymentMethodObj)
	_ = pluginObj.Set("virtualInventory", virtualInventoryObj)
	_ = pluginObj.Set("virtualInventoryBinding", virtualInventoryBindingObj)
	_ = pluginObj.Set("market", marketObj)
	_ = pluginObj.Set("emailTemplate", emailTemplateObj)
	_ = pluginObj.Set("landingPage", landingPageObj)
	_ = pluginObj.Set("invoiceTemplate", invoiceTemplateObj)
	_ = pluginObj.Set("authBranding", authBrandingObj)
	_ = pluginObj.Set("pageRulePack", pageRulePackObj)

	fsObj := vm.NewObject()
	_ = fsObj.Set("enabled", opts.allowFS)
	_ = fsObj.Set("root", "/")
	_ = fsObj.Set("codeRoot", filepath.ToSlash(fsCtx.CodeRoot))
	_ = fsObj.Set("dataRoot", filepath.ToSlash(fsCtx.DataRoot))
	_ = fsObj.Set("pluginID", fsCtx.PluginID)
	_ = fsObj.Set("pluginName", fsCtx.PluginName)
	_ = fsObj.Set("maxFiles", opts.fsMaxFiles)
	_ = fsObj.Set("maxTotalBytes", opts.fsMaxTotalBytes)
	_ = fsObj.Set("maxReadBytes", opts.fsMaxReadBytes)
	_ = fsObj.Set("exists", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		ok, err := fsState.Exists(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(ok)
	})
	_ = fsObj.Set("readText", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.fs.readText(path) requires path"))
		}
		content, err := fsState.ReadText(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(content)
	})
	_ = fsObj.Set("readBase64", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.fs.readBase64(path) requires path"))
		}
		content, err := fsState.ReadBase64(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(content)
	})
	_ = fsObj.Set("readJSON", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.fs.readJSON(path) requires path"))
		}
		content, err := fsState.ReadJSON(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(content)
	})
	_ = fsObj.Set("writeText", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 2 {
			throwJSError(fmt.Errorf("Plugin.fs.writeText(path, content) requires path and content"))
		}
		if err := fsState.WriteText(call.Arguments[0].String(), call.Arguments[1].String()); err != nil {
			throwJSError(err)
		}
		return vm.ToValue(true)
	})
	_ = fsObj.Set("writeJSON", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 2 {
			throwJSError(fmt.Errorf("Plugin.fs.writeJSON(path, value) requires path and value"))
		}
		if err := fsState.WriteJSON(call.Arguments[0].String(), exportGojaValue(call.Arguments[1])); err != nil {
			throwJSError(err)
		}
		return vm.ToValue(true)
	})
	_ = fsObj.Set("writeBase64", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 2 {
			throwJSError(fmt.Errorf("Plugin.fs.writeBase64(path, base64) requires path and base64 payload"))
		}
		if err := fsState.WriteBase64(call.Arguments[0].String(), call.Arguments[1].String()); err != nil {
			throwJSError(err)
		}
		return vm.ToValue(true)
	})
	_ = fsObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			return vm.ToValue(false)
		}
		ok, err := fsState.Delete(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(ok)
	})
	_ = fsObj.Set("mkdir", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.fs.mkdir(path) requires path"))
		}
		if err := fsState.MkdirAll(call.Arguments[0].String()); err != nil {
			throwJSError(err)
		}
		return vm.ToValue(true)
	})
	_ = fsObj.Set("list", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		target := "."
		if len(call.Arguments) > 0 {
			target = call.Arguments[0].String()
		}
		items, err := fsState.List(target)
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(items)
	})
	_ = fsObj.Set("stat", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		if len(call.Arguments) < 1 {
			throwJSError(fmt.Errorf("Plugin.fs.stat(path) requires path"))
		}
		stat, err := fsState.Stat(call.Arguments[0].String())
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(stat)
	})
	_ = fsObj.Set("usage", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		usage, err := fsState.Usage()
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(pluginFSUsageToMap(usage))
	})
	_ = fsObj.Set("recalculateUsage", func(call goja.FunctionCall) goja.Value {
		fsState := requireFS()
		usage, err := fsState.RecalculateUsage()
		if err != nil {
			throwJSError(err)
		}
		return vm.ToValue(pluginFSUsageToMap(usage))
	})
	_ = pluginObj.Set("fs", fsObj)

	vm.Set("Plugin", pluginObj)
}

func installBrowserCompatibilityPolyfills(vm *goja.Runtime) {
	installURLSearchParamsPolyfill(vm)
	installWebEncodingPolyfills(vm)
	installRuntimeAsyncCompatibilityGlobals(vm, runtimeAsyncGlobalHooks{
		StructuredClone: func(value goja.Value) (goja.Value, error) {
			return runtimeStructuredCloneValue(vm, value)
		},
	})
}

func installURLSearchParamsPolyfill(vm *goja.Runtime) {
	if vm == nil {
		return
	}
	existing := vm.Get("URLSearchParams")
	if existing != nil && !goja.IsUndefined(existing) && !goja.IsNull(existing) {
		return
	}
	if _, err := vm.RunString(urlSearchParamsPolyfillSource); err != nil {
		panic(vm.NewGoError(fmt.Errorf("register URLSearchParams polyfill: %w", err)))
	}
}

func installWebEncodingPolyfills(vm *goja.Runtime) {
	if vm == nil {
		return
	}
	if _, err := vm.RunString(webEncodingPolyfillSource); err != nil {
		panic(vm.NewGoError(fmt.Errorf("register web encoding polyfills: %w", err)))
	}
}

func exportPluginHostArgument(arguments []goja.Value, index int) interface{} {
	if index < 0 || index >= len(arguments) {
		return nil
	}
	return exportGojaValue(arguments[index])
}

func buildPluginHostObjectParams(value interface{}, defaultKey string) map[string]interface{} {
	params := normalizePluginHostParams(value)
	if len(params) > 0 {
		return params
	}
	if value == nil {
		return map[string]interface{}{}
	}
	normalizedKey := strings.TrimSpace(defaultKey)
	if normalizedKey == "" {
		normalizedKey = "id"
	}
	return map[string]interface{}{normalizedKey: value}
}

func normalizePluginHostParams(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case nil:
		return map[string]interface{}{}
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" {
				continue
			}
			result[normalizedKey] = item
		}
		return result
	default:
		return map[string]interface{}{}
	}
}

func buildSharedPluginHostRootObjects(
	vm *goja.Runtime,
	hostInvoker func(action string, params map[string]interface{}) map[string]interface{},
) map[string]*goja.Object {
	roots := map[string]*goja.Object{}
	for _, definition := range pluginhost.ListSharedActionDefinitions() {
		if len(definition.JSImportPath) < 2 {
			continue
		}

		rootKey := strings.TrimSpace(definition.JSImportPath[0])
		if rootKey == "" {
			continue
		}
		rootObj := roots[rootKey]
		if rootObj == nil {
			rootObj = vm.NewObject()
			roots[rootKey] = rootObj
		}

		current := rootObj
		for _, segment := range definition.JSImportPath[1 : len(definition.JSImportPath)-1] {
			key := strings.TrimSpace(segment)
			if key == "" {
				current = nil
				break
			}
			nextValue := current.Get(key)
			if nextValue == nil || goja.IsUndefined(nextValue) || goja.IsNull(nextValue) {
				nextObj := vm.NewObject()
				_ = current.Set(key, nextObj)
				current = nextObj
				continue
			}
			current = nextValue.ToObject(vm)
		}
		if current == nil {
			continue
		}

		methodKey := strings.TrimSpace(definition.JSImportPath[len(definition.JSImportPath)-1])
		action := definition.Action
		if methodKey == "" || strings.TrimSpace(action) == "" {
			continue
		}
		_ = current.Set(methodKey, func(call goja.FunctionCall) goja.Value {
			params := normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
			return vm.ToValue(hostInvoker(action, params))
		})
	}
	return roots
}

func performPluginHostRequest(hostCfg *pluginipc.HostAPIConfig, action string, params map[string]interface{}) (map[string]interface{}, error) {
	if hostCfg == nil {
		return nil, fmt.Errorf("Plugin.host is unavailable")
	}

	network := strings.ToLower(strings.TrimSpace(hostCfg.Network))
	address := strings.TrimSpace(hostCfg.Address)
	accessToken := strings.TrimSpace(hostCfg.AccessToken)
	if network == "" || address == "" || accessToken == "" {
		return nil, fmt.Errorf("Plugin.host is unavailable")
	}
	if err := pluginutil.ValidateJSWorkerSocketEndpoint(network, address); err != nil {
		return nil, fmt.Errorf("Plugin.host endpoint is invalid: %w", err)
	}

	trimmedAction := strings.TrimSpace(action)
	if trimmedAction == "" {
		return nil, fmt.Errorf("Plugin.host action is required")
	}
	if session, ok := loadPluginHostSession(hostCfg); ok {
		return session.request(hostCfg, trimmedAction, params)
	}
	return performPluginHostRequestOnce(hostCfg, network, address, accessToken, trimmedAction, params)
}

func performPluginHostRequestOnce(
	hostCfg *pluginipc.HostAPIConfig,
	network string,
	address string,
	accessToken string,
	action string,
	params map[string]interface{},
) (map[string]interface{}, error) {
	timeout := resolvePluginHostRequestTimeout(hostCfg, action, params)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("Plugin.host dial failed: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout + 2*time.Second))

	req := pluginipc.HostRequest{
		AccessToken: accessToken,
		Action:      action,
		Params:      normalizePluginHostParams(params),
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Plugin.host encode request failed: %w", err)
	}

	var resp pluginipc.HostResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Plugin.host decode response failed: %w", err)
	}
	if !resp.Success {
		errMsg := strings.TrimSpace(resp.Error)
		if errMsg == "" {
			if resp.Status > 0 {
				errMsg = fmt.Sprintf("Plugin.host request failed with status %d", resp.Status)
			} else {
				errMsg = "Plugin.host request failed"
			}
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	if resp.Data == nil {
		return map[string]interface{}{}, nil
	}
	return resp.Data, nil
}

func attachPluginHostSession(hostCfg *pluginipc.HostAPIConfig) func() {
	if hostCfg == nil {
		return func() {}
	}
	network := strings.ToLower(strings.TrimSpace(hostCfg.Network))
	address := strings.TrimSpace(hostCfg.Address)
	accessToken := strings.TrimSpace(hostCfg.AccessToken)
	if network == "" || address == "" || accessToken == "" {
		return func() {}
	}
	if err := pluginutil.ValidateJSWorkerSocketEndpoint(network, address); err != nil {
		return func() {}
	}
	session := &pluginHostSession{
		network:     network,
		address:     address,
		accessToken: accessToken,
	}
	pluginHostSessions.Store(hostCfg, session)
	return func() {
		pluginHostSessions.Delete(hostCfg)
		session.close()
	}
}

func loadPluginHostSession(hostCfg *pluginipc.HostAPIConfig) (*pluginHostSession, bool) {
	if hostCfg == nil {
		return nil, false
	}
	raw, ok := pluginHostSessions.Load(hostCfg)
	if !ok || raw == nil {
		return nil, false
	}
	session, ok := raw.(*pluginHostSession)
	if !ok || session == nil {
		return nil, false
	}
	return session, true
}

func (s *pluginHostSession) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
}

func (s *pluginHostSession) closeLocked() {
	if s == nil {
		return
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.conn = nil
	s.encoder = nil
	s.decoder = nil
}

func (s *pluginHostSession) ensureConnLocked(timeout time.Duration) error {
	if s == nil {
		return fmt.Errorf("Plugin.host is unavailable")
	}
	if s.conn != nil && s.encoder != nil && s.decoder != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, s.network, s.address)
	if err != nil {
		return fmt.Errorf("Plugin.host dial failed: %w", err)
	}
	s.conn = conn
	s.encoder = json.NewEncoder(conn)
	s.decoder = json.NewDecoder(conn)
	return nil
}

func (s *pluginHostSession) request(hostCfg *pluginipc.HostAPIConfig, action string, params map[string]interface{}) (map[string]interface{}, error) {
	if s == nil {
		return nil, fmt.Errorf("Plugin.host is unavailable")
	}
	timeout := resolvePluginHostRequestTimeout(hostCfg, action, params)
	req := pluginipc.HostRequest{
		AccessToken: s.accessToken,
		Action:      action,
		Params:      normalizePluginHostParams(params),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureConnLocked(timeout); err != nil {
		return nil, err
	}
	_ = s.conn.SetDeadline(time.Now().Add(timeout + 2*time.Second))
	if err := s.encoder.Encode(req); err != nil {
		s.closeLocked()
		return nil, fmt.Errorf("Plugin.host encode request failed: %w", err)
	}
	var resp pluginipc.HostResponse
	if err := s.decoder.Decode(&resp); err != nil {
		s.closeLocked()
		return nil, fmt.Errorf("Plugin.host decode response failed: %w", err)
	}
	if !resp.Success {
		errMsg := strings.TrimSpace(resp.Error)
		if errMsg == "" {
			if resp.Status > 0 {
				errMsg = fmt.Sprintf("Plugin.host request failed with status %d", resp.Status)
			} else {
				errMsg = "Plugin.host request failed"
			}
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	if resp.Data == nil {
		return map[string]interface{}{}, nil
	}
	return resp.Data, nil
}

func resolvePluginHostTimeoutMs(hostCfg *pluginipc.HostAPIConfig) int {
	if hostCfg == nil || hostCfg.TimeoutMs <= 0 {
		return 30000
	}
	if hostCfg.TimeoutMs < 100 {
		return 100
	}
	return hostCfg.TimeoutMs
}

func resolvePluginHostRequestTimeout(hostCfg *pluginipc.HostAPIConfig, action string, params map[string]interface{}) time.Duration {
	timeoutMs := resolvePluginHostTimeoutMs(hostCfg)
	if override := parsePluginHostRequestTimeoutMs(params); override > timeoutMs {
		timeoutMs = override
	}
	if strings.EqualFold(strings.TrimSpace(action), "host.workspace.read_input") && timeoutMs < workerWorkspaceHostInputTimeoutMs {
		timeoutMs = workerWorkspaceHostInputTimeoutMs
	}
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func parsePluginHostRequestTimeoutMs(params map[string]interface{}) int {
	if len(params) == 0 {
		return 0
	}
	for _, key := range []string{"timeout_ms", "timeoutMs"} {
		value, exists := params[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case int:
			if typed > 0 {
				return typed
			}
		case int64:
			if typed > 0 {
				return int(typed)
			}
		case float64:
			if typed > 0 {
				return int(typed)
			}
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func newPluginStreamWriter(
	vm *goja.Runtime,
	emit func(map[string]interface{}, map[string]string) error,
) *goja.Object {
	stream := vm.NewObject()

	writeChunk := func(data interface{}, metadata interface{}) goja.Value {
		if emit == nil {
			return goja.Undefined()
		}
		if err := emit(
			normalizePluginStreamChunkData(data),
			normalizePluginHTTPHeaders(metadata),
		); err != nil {
			panic(vm.NewGoError(err))
		}
		return goja.Undefined()
	}

	_ = stream.Set("write", func(call goja.FunctionCall) goja.Value {
		var data interface{}
		if len(call.Arguments) > 0 {
			data = exportGojaValue(call.Arguments[0])
		}
		var metadata interface{}
		if len(call.Arguments) > 1 {
			metadata = exportGojaValue(call.Arguments[1])
		}
		return writeChunk(data, metadata)
	})

	_ = stream.Set("emit", func(call goja.FunctionCall) goja.Value {
		var data interface{}
		if len(call.Arguments) > 0 {
			data = exportGojaValue(call.Arguments[0])
		}
		var metadata interface{}
		if len(call.Arguments) > 1 {
			metadata = exportGojaValue(call.Arguments[1])
		}
		return writeChunk(data, metadata)
	})

	_ = stream.Set("progress", func(call goja.FunctionCall) goja.Value {
		status := ""
		if len(call.Arguments) > 0 {
			status = strings.TrimSpace(call.Arguments[0].String())
		}
		payload := map[string]interface{}{}
		if status != "" {
			payload["status"] = status
		}
		if len(call.Arguments) > 1 {
			if progressValue, ok := normalizePluginStreamProgressValue(exportGojaValue(call.Arguments[1])); ok {
				payload["progress"] = progressValue
			}
		}
		var metadata interface{}
		if len(call.Arguments) > 2 {
			metadata = exportGojaValue(call.Arguments[2])
		}
		return writeChunk(payload, metadata)
	})

	return stream
}

type pluginHTTPRequestOptions struct {
	URL       string
	Method    string
	Headers   map[string]string
	Body      interface{}
	TimeoutMs int
}

func decodePluginHTTPRequestOptions(value interface{}) pluginHTTPRequestOptions {
	record, _ := value.(map[string]interface{})
	if record == nil {
		return pluginHTTPRequestOptions{}
	}
	options := pluginHTTPRequestOptions{
		URL:     strings.TrimSpace(interfaceToString(record["url"])),
		Method:  strings.ToUpper(strings.TrimSpace(interfaceToString(record["method"]))),
		Headers: normalizePluginHTTPHeaders(record["headers"]),
		Body:    record["body"],
	}
	switch typed := record["timeout_ms"].(type) {
	case int:
		options.TimeoutMs = typed
	case int32:
		options.TimeoutMs = int(typed)
	case int64:
		options.TimeoutMs = int(typed)
	case float64:
		options.TimeoutMs = int(typed)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			options.TimeoutMs = parsed
		}
	}
	return options
}

func performPluginHTTPRequest(opts workerOptions, request pluginHTTPRequestOptions) map[string]interface{} {
	start := time.Now()
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}
	urlStr := strings.TrimSpace(request.URL)
	if urlStr == "" {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, fmt.Errorf("url is required"))
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, fmt.Errorf("invalid url: %w", err))
	}
	if err := validatePluginHTTPURL(parsedURL); err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, fmt.Errorf("url is not allowed: %w", err))
	}

	timeoutMs := clampPluginHTTPTimeoutMs(request.TimeoutMs, opts.timeoutMs)
	reqBody, contentType, err := buildPluginHTTPRequestBody(request.Body)
	if err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), reqBody)
	if err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, fmt.Errorf("create request failed: %w", err))
	}

	req.Header.Set("User-Agent", pluginHTTPUserAgent)
	if contentType != "" && !hasPluginHTTPHeader(request.Headers, "Content-Type") {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range request.Headers {
		if shouldDropPluginHTTPHeader(key) {
			continue
		}
		req.Header.Set(key, value)
	}

	client := newPluginHTTPClient(time.Duration(timeoutMs) * time.Millisecond)
	resp, err := client.Do(req)
	if err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, 0, nil, fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()

	responseHeaders := normalizePluginHTTPResponseHeaders(resp.Header)
	limitedReader := io.LimitReader(resp.Body, maxPluginHTTPResponseBytes+1)
	payload, err := io.ReadAll(limitedReader)
	if err != nil {
		return buildPluginHTTPErrorResult(urlStr, start, resp.StatusCode, responseHeaders, fmt.Errorf("read response failed: %w", err))
	}
	if int64(len(payload)) > maxPluginHTTPResponseBytes {
		return buildPluginHTTPErrorResult(
			urlStr,
			start,
			resp.StatusCode,
			responseHeaders,
			fmt.Errorf("response exceeds max_response_bytes (%d)", maxPluginHTTPResponseBytes),
		)
	}

	finalURL := parsedURL.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	result := map[string]interface{}{
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"url":         finalURL,
		"status":      resp.StatusCode,
		"statusText":  resp.Status,
		"headers":     responseHeaders,
		"body":        string(payload),
		"duration_ms": int(time.Since(start) / time.Millisecond),
		"redirected":  finalURL != parsedURL.String(),
	}

	respContentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(respContentType, "application/json") {
		var decoded interface{}
		if err := json.Unmarshal(payload, &decoded); err == nil {
			result["data"] = decoded
		}
	}

	log.Printf("[jsworker][plugin.http] %s %s -> %d (%dms)", method, parsedURL.String(), resp.StatusCode, result["duration_ms"])
	return result
}

func buildPluginHTTPErrorResult(
	urlStr string,
	start time.Time,
	status int,
	headers map[string]string,
	err error,
) map[string]interface{} {
	if headers == nil {
		headers = map[string]string{}
	}
	result := map[string]interface{}{
		"ok":          false,
		"url":         urlStr,
		"status":      status,
		"statusText":  "",
		"headers":     headers,
		"body":        "",
		"error":       err.Error(),
		"duration_ms": int(time.Since(start) / time.Millisecond),
	}
	log.Printf("[jsworker][plugin.http] request failed url=%s status=%d err=%v", urlStr, status, err)
	return result
}

func clampPluginHTTPTimeoutMs(requestedMs int, runtimeTimeoutMs int) int {
	maxAllowed := runtimeTimeoutMs
	if maxAllowed <= 0 {
		maxAllowed = defaultPluginHTTPTimeoutMs
	}
	if requestedMs <= 0 {
		if maxAllowed < defaultPluginHTTPTimeoutMs {
			return maxAllowed
		}
		return defaultPluginHTTPTimeoutMs
	}
	if requestedMs > maxAllowed {
		requestedMs = maxAllowed
	}
	if requestedMs < 100 {
		return 100
	}
	return requestedMs
}

func buildPluginHTTPRequestBody(body interface{}) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	switch typed := body.(type) {
	case string:
		contentType := "text/plain"
		trimmed := strings.TrimSpace(typed)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			contentType = "application/json"
		}
		return strings.NewReader(typed), contentType, nil
	case map[string]interface{}, []interface{}:
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil, "", fmt.Errorf("encode request body failed: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil, "", fmt.Errorf("encode request body failed: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	}
}

func normalizePluginHTTPHeaders(value interface{}) map[string]string {
	out := map[string]string{}
	switch typed := value.(type) {
	case map[string]string:
		for key, item := range typed {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			out[trimmedKey] = item
		}
	case map[string]interface{}:
		for key, item := range typed {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			out[trimmedKey] = interfaceToString(item)
		}
	}
	return out
}

func normalizePluginHTTPResponseHeaders(values http.Header) map[string]string {
	out := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		out[key] = items[0]
	}
	return out
}

func hasPluginHTTPHeader(headers map[string]string, key string) bool {
	for item := range headers {
		if strings.EqualFold(strings.TrimSpace(item), key) {
			return true
		}
	}
	return false
}

func shouldDropPluginHTTPHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "host", "proxy-authorization", "content-length", "transfer-encoding":
		return true
	default:
		return false
	}
}

func newPluginHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = time.Duration(defaultPluginHTTPTimeoutMs) * time.Millisecond
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     sharedPluginHTTPTransport,
		CheckRedirect: pluginHTTPCheckRedirect,
	}
}

func pluginHTTPDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{}
	if ip, err := netip.ParseAddr(host); err == nil {
		if isBlockedPluginHTTPIP(ip) {
			return nil, fmt.Errorf("blocked address")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	if isBlockedPluginHTTPHostname(host) {
		return nil, fmt.Errorf("blocked host")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, ip := range ips {
		if isBlockedPluginHTTPIP(ip) {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("blocked address")
	}
	return nil, lastErr
}

func pluginHTTPCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxPluginHTTPRedirects {
		return fmt.Errorf("stopped after too many redirects")
	}
	if req.URL == nil {
		return fmt.Errorf("invalid redirect url")
	}
	return validatePluginHTTPURL(req.URL)
}

func validatePluginHTTPURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if isBlockedPluginHTTPIP(ip) {
			return fmt.Errorf("blocked address")
		}
		return nil
	}
	if isBlockedPluginHTTPHostname(host) {
		return fmt.Errorf("blocked host")
	}
	return nil
}

func isBlockedPluginHTTPHostname(host string) bool {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "localhost" || strings.HasSuffix(normalized, ".localhost") {
		return true
	}
	return strings.HasSuffix(normalized, ".local") || strings.HasSuffix(normalized, ".internal")
}

func isBlockedPluginHTTPIP(ip netip.Addr) bool {
	normalized := ip.Unmap()
	return normalized.IsLoopback() ||
		normalized.IsPrivate() ||
		normalized.IsLinkLocalUnicast() ||
		normalized.IsLinkLocalMulticast() ||
		normalized.IsMulticast() ||
		normalized.IsUnspecified() ||
		pluginHTTPCGNATPrefix.Contains(normalized)
}

func interfaceToString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

var pluginFSLocks sync.Map
var pluginFSUsageCaches sync.Map

type pluginFS struct {
	codeRoot      string
	dataRoot      string
	maxFiles      int
	maxTotalBytes int64
	maxReadBytes  int64
	lock          *sync.Mutex
	usageCache    *pluginFSUsageCache
}

type pluginFSUsage struct {
	FileCount  int   `json:"file_count"`
	TotalBytes int64 `json:"total_bytes"`
	MaxFiles   int   `json:"max_files"`
	MaxBytes   int64 `json:"max_bytes"`
}

func pluginFSUsageToMap(usage pluginFSUsage) map[string]interface{} {
	return map[string]interface{}{
		"file_count":  usage.FileCount,
		"total_bytes": usage.TotalBytes,
		"max_files":   usage.MaxFiles,
		"max_bytes":   usage.MaxBytes,
	}
}

type pluginFSUsageSnapshot struct {
	FileCount  int
	TotalBytes int64
}

type pluginFSUsageCache struct {
	mu          sync.Mutex
	initialized bool
	fileCount   int
	totalBytes  int64
}

func newPluginFS(ctx pluginFSRuntimeContext, maxFiles int, maxTotalBytes int64, maxReadBytes int64) (*pluginFS, error) {
	codeRoot := filepath.Clean(filepath.FromSlash(strings.TrimSpace(ctx.CodeRoot)))
	dataRoot := filepath.Clean(filepath.FromSlash(strings.TrimSpace(ctx.DataRoot)))
	if codeRoot == "" || codeRoot == "." {
		return nil, fmt.Errorf("plugin fs code root is empty")
	}
	if !filepath.IsAbs(codeRoot) {
		abs, err := filepath.Abs(codeRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve plugin fs code root failed: %w", err)
		}
		codeRoot = filepath.Clean(abs)
	}
	if dataRoot == "" || dataRoot == "." {
		return nil, fmt.Errorf("plugin fs data root is empty")
	}
	if !filepath.IsAbs(dataRoot) {
		abs, err := filepath.Abs(dataRoot)
		if err != nil {
			return nil, fmt.Errorf("resolve plugin fs data root failed: %w", err)
		}
		dataRoot = filepath.Clean(abs)
	}

	info, err := os.Stat(codeRoot)
	if err != nil {
		return nil, fmt.Errorf("plugin fs code root is unavailable: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugin fs code root is not directory: %s", filepath.ToSlash(codeRoot))
	}
	if err := os.MkdirAll(dataRoot, 0755); err != nil {
		return nil, fmt.Errorf("prepare plugin fs data root failed: %w", err)
	}
	if maxFiles <= 0 {
		maxFiles = 2048
	}
	if maxTotalBytes <= 0 {
		maxTotalBytes = 128 * 1024 * 1024
	}
	if maxReadBytes <= 0 {
		maxReadBytes = 4 * 1024 * 1024
	}
	if maxReadBytes > maxTotalBytes {
		maxReadBytes = maxTotalBytes
	}
	return &pluginFS{
		codeRoot:      codeRoot,
		dataRoot:      dataRoot,
		maxFiles:      maxFiles,
		maxTotalBytes: maxTotalBytes,
		maxReadBytes:  maxReadBytes,
		lock:          pluginFSLockForRoot(dataRoot),
		usageCache:    pluginFSUsageCacheForRoot(dataRoot),
	}, nil
}

func pluginFSLockForRoot(root string) *sync.Mutex {
	normalized := filepath.Clean(root)
	if existing, ok := pluginFSLocks.Load(normalized); ok {
		if lock, ok := existing.(*sync.Mutex); ok {
			return lock
		}
	}
	lock := &sync.Mutex{}
	actual, _ := pluginFSLocks.LoadOrStore(normalized, lock)
	cast, _ := actual.(*sync.Mutex)
	if cast == nil {
		return lock
	}
	return cast
}

func pluginFSUsageCacheForRoot(root string) *pluginFSUsageCache {
	normalized := filepath.Clean(root)
	if existing, ok := pluginFSUsageCaches.Load(normalized); ok {
		if cache, ok := existing.(*pluginFSUsageCache); ok {
			return cache
		}
	}
	cache := &pluginFSUsageCache{}
	actual, _ := pluginFSUsageCaches.LoadOrStore(normalized, cache)
	resolved, _ := actual.(*pluginFSUsageCache)
	if resolved == nil {
		return cache
	}
	return resolved
}

func (p *pluginFS) Exists(path string) (bool, error) {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return false, err
	}
	entry, err := p.lookupEntry(resolvedPath)
	if err != nil {
		return false, err
	}
	return entry != nil, nil
}

func (p *pluginFS) ReadText(path string) (string, error) {
	body, err := p.readBytes(path)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (p *pluginFS) ReadBase64(path string) (string, error) {
	body, err := p.readBytes(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

func (p *pluginFS) ReadJSON(path string) (interface{}, error) {
	body, err := p.readBytes(path)
	if err != nil {
		return nil, err
	}
	var decoded interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("read json failed: %w", err)
	}
	return decoded, nil
}

func (p *pluginFS) WriteText(path string, content string) error {
	return p.writeBytes(path, []byte(content))
}

func (p *pluginFS) WriteJSON(path string, value interface{}) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json failed: %w", err)
	}
	return p.writeBytes(path, body)
}

func (p *pluginFS) WriteBase64(path string, raw string) error {
	payload := strings.TrimSpace(raw)
	if idx := strings.Index(payload, ","); idx >= 0 && strings.Contains(strings.ToLower(payload[:idx]), "base64") {
		payload = payload[idx+1:]
	}
	body, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("decode base64 failed: %w", err)
	}
	return p.writeBytes(path, body)
}

func (p *pluginFS) Delete(path string) (bool, error) {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return false, err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	info, statErr := os.Stat(resolvedPath.DataPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, fmt.Errorf("stat delete target failed: %w", statErr)
	}

	removed, err := scanPluginFSUsageFromPath(resolvedPath.DataPath, info)
	if err != nil {
		return false, fmt.Errorf("collect delete usage failed: %w", err)
	}

	if info.IsDir() {
		if err := os.RemoveAll(resolvedPath.DataPath); err != nil {
			return false, fmt.Errorf("delete directory failed: %w", err)
		}
	} else if err := os.Remove(resolvedPath.DataPath); err != nil {
		return false, fmt.Errorf("delete file failed: %w", err)
	}

	if removed.FileCount > 0 || removed.TotalBytes > 0 {
		p.usageCache.applyDelta(-removed.FileCount, -removed.TotalBytes)
	}
	return true, nil
}

func (p *pluginFS) MkdirAll(path string) error {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return err
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	return os.MkdirAll(resolvedPath.DataPath, 0755)
}

func (p *pluginFS) List(path string) ([]map[string]interface{}, error) {
	resolvedPath, err := p.resolvePathPair(path, true)
	if err != nil {
		return nil, err
	}

	codeDirExists := false
	dataDirExists := false
	if info, err := os.Stat(resolvedPath.CodePath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("list target is not directory")
		}
		codeDirExists = true
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if info, err := os.Stat(resolvedPath.DataPath); err == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("list target is not directory")
		}
		dataDirExists = true
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if !codeDirExists && !dataDirExists {
		return []map[string]interface{}{}, nil
	}

	type listEntry struct {
		path string
	}
	entries := make(map[string]listEntry)
	addEntries := func(dir string) error {
		readEntries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range readEntries {
			entries[entry.Name()] = listEntry{path: filepath.Join(dir, entry.Name())}
		}
		return nil
	}
	if codeDirExists {
		if err := addEntries(resolvedPath.CodePath); err != nil {
			return nil, fmt.Errorf("list code layer failed: %w", err)
		}
	}
	if dataDirExists {
		if err := addEntries(resolvedPath.DataPath); err != nil {
			return nil, fmt.Errorf("list data layer failed: %w", err)
		}
	}

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	items := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		entryPath := entries[name].path
		info, err := os.Stat(entryPath)
		if err != nil {
			continue
		}
		relPath := filepath.Clean(filepath.Join(resolvedPath.RelPath, name))
		if relPath == "." {
			relPath = name
		}
		item := map[string]interface{}{
			"name":     name,
			"path":     filepath.ToSlash(relPath),
			"is_dir":   info.IsDir(),
			"size":     info.Size(),
			"mod_time": info.ModTime().UTC().Format(time.RFC3339),
		}
		items = append(items, item)
	}
	return items, nil
}

func (p *pluginFS) Stat(path string) (map[string]interface{}, error) {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return nil, err
	}

	entry, err := p.lookupEntry(resolvedPath)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return map[string]interface{}{
			"exists": false,
			"path":   filepath.ToSlash(resolvedPath.RelPath),
		}, nil
	}
	return map[string]interface{}{
		"exists":   true,
		"name":     entry.Info.Name(),
		"path":     filepath.ToSlash(resolvedPath.RelPath),
		"is_dir":   entry.Info.IsDir(),
		"size":     entry.Info.Size(),
		"mod_time": entry.Info.ModTime().UTC().Format(time.RFC3339),
		"layer":    entry.Layer,
	}, nil
}

func (p *pluginFS) Usage() (pluginFSUsage, error) {
	return p.currentUsage()
}

func (p *pluginFS) RecalculateUsage() (pluginFSUsage, error) {
	snapshot, err := scanPluginFSUsageRoot(p.dataRoot)
	if err != nil {
		return pluginFSUsage{}, err
	}
	p.usageCache.set(snapshot)
	return p.snapshotToUsage(snapshot), nil
}

func (p *pluginFS) readBytes(path string) ([]byte, error) {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return nil, err
	}

	entry, err := p.lookupEntry(resolvedPath)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, os.ErrNotExist
	}
	if entry.Info.IsDir() {
		return nil, fmt.Errorf("path is directory")
	}
	if entry.Info.Size() > p.maxReadBytes {
		return nil, fmt.Errorf("read exceeds max_read_bytes (%d)", p.maxReadBytes)
	}
	body, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > p.maxReadBytes {
		return nil, fmt.Errorf("read exceeds max_read_bytes (%d)", p.maxReadBytes)
	}
	return body, nil
}

func (p *pluginFS) writeBytes(path string, body []byte) error {
	resolvedPath, err := p.resolvePathPair(path, false)
	if err != nil {
		return err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	existingSize := int64(0)
	exists := false
	if info, statErr := os.Stat(resolvedPath.DataPath); statErr == nil {
		if info.IsDir() {
			return fmt.Errorf("target path is directory")
		}
		exists = true
		existingSize = info.Size()
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("stat existing file failed: %w", statErr)
	}

	snapshot, err := p.ensureUsageSnapshot()
	if err != nil {
		return err
	}
	nextFiles := snapshot.FileCount
	if !exists {
		nextFiles++
	}
	nextTotal := snapshot.TotalBytes - existingSize + int64(len(body))
	if nextFiles > p.maxFiles {
		return fmt.Errorf("plugin fs quota exceeded: max_files=%d", p.maxFiles)
	}
	if nextTotal > p.maxTotalBytes {
		return fmt.Errorf("plugin fs quota exceeded: max_total_bytes=%d", p.maxTotalBytes)
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath.DataPath), 0755); err != nil {
		return fmt.Errorf("create parent directory failed: %w", err)
	}
	if err := os.WriteFile(resolvedPath.DataPath, body, 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	filesDelta := 0
	if !exists {
		filesDelta = 1
	}
	bytesDelta := int64(len(body)) - existingSize
	if filesDelta != 0 || bytesDelta != 0 {
		p.usageCache.applyDelta(filesDelta, bytesDelta)
	}
	return nil
}

func (p *pluginFS) currentUsage() (pluginFSUsage, error) {
	snapshot, err := p.ensureUsageSnapshot()
	if err != nil {
		return pluginFSUsage{}, err
	}
	return p.snapshotToUsage(snapshot), nil
}

func (p *pluginFS) ensureUsageSnapshot() (pluginFSUsageSnapshot, error) {
	if p.usageCache == nil {
		return scanPluginFSUsageRoot(p.dataRoot)
	}
	if snapshot, ok := p.usageCache.snapshot(); ok {
		return snapshot, nil
	}
	snapshot, err := scanPluginFSUsageRoot(p.dataRoot)
	if err != nil {
		return pluginFSUsageSnapshot{}, err
	}
	p.usageCache.set(snapshot)
	return snapshot, nil
}

func (p *pluginFS) snapshotToUsage(snapshot pluginFSUsageSnapshot) pluginFSUsage {
	return pluginFSUsage{
		FileCount:  snapshot.FileCount,
		TotalBytes: snapshot.TotalBytes,
		MaxFiles:   p.maxFiles,
		MaxBytes:   p.maxTotalBytes,
	}
}

func scanPluginFSUsageRoot(root string) (pluginFSUsageSnapshot, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return pluginFSUsageSnapshot{}, nil
		}
		return pluginFSUsageSnapshot{}, err
	}
	return scanPluginFSUsageFromPath(root, nil)
}

func scanPluginFSUsageFromPath(path string, info os.FileInfo) (pluginFSUsageSnapshot, error) {
	if info == nil {
		statInfo, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return pluginFSUsageSnapshot{}, nil
			}
			return pluginFSUsageSnapshot{}, err
		}
		info = statInfo
	}

	if !info.IsDir() {
		if info.Mode()&os.ModeSymlink != 0 {
			return pluginFSUsageSnapshot{}, nil
		}
		return pluginFSUsageSnapshot{
			FileCount:  1,
			TotalBytes: info.Size(),
		}, nil
	}

	snapshot := pluginFSUsageSnapshot{}
	err := filepath.WalkDir(path, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		snapshot.FileCount++
		snapshot.TotalBytes += fileInfo.Size()
		return nil
	})
	if err != nil {
		return pluginFSUsageSnapshot{}, err
	}
	return snapshot, nil
}

func (c *pluginFSUsageCache) snapshot() (pluginFSUsageSnapshot, bool) {
	if c == nil {
		return pluginFSUsageSnapshot{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.initialized {
		return pluginFSUsageSnapshot{}, false
	}
	return pluginFSUsageSnapshot{
		FileCount:  c.fileCount,
		TotalBytes: c.totalBytes,
	}, true
}

func (c *pluginFSUsageCache) set(snapshot pluginFSUsageSnapshot) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.initialized = true
	c.fileCount = snapshot.FileCount
	c.totalBytes = snapshot.TotalBytes
	c.mu.Unlock()
}

func (c *pluginFSUsageCache) applyDelta(fileDelta int, bytesDelta int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if !c.initialized {
		c.mu.Unlock()
		return
	}
	c.fileCount += fileDelta
	if c.fileCount < 0 {
		c.fileCount = 0
	}
	c.totalBytes += bytesDelta
	if c.totalBytes < 0 {
		c.totalBytes = 0
	}
	c.mu.Unlock()
}

type pluginFSResolvedPath struct {
	RelPath  string
	DataPath string
	CodePath string
}

type pluginFSEntry struct {
	Path  string
	Info  os.FileInfo
	Layer string
}

func (p *pluginFS) resolvePathPair(path string, allowEmpty bool) (pluginFSResolvedPath, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if allowEmpty {
			trimmed = "."
		} else {
			return pluginFSResolvedPath{}, fmt.Errorf("path is required")
		}
	}
	normalized := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(normalized) {
		return pluginFSResolvedPath{}, fmt.Errorf("absolute path is not allowed: %s", filepath.ToSlash(path))
	}
	if normalized == ".." {
		return pluginFSResolvedPath{}, fmt.Errorf("path outside plugin root: %s", filepath.ToSlash(path))
	}
	parentPrefix := ".." + string(os.PathSeparator)
	if strings.HasPrefix(normalized, parentPrefix) {
		return pluginFSResolvedPath{}, fmt.Errorf("path outside plugin root: %s", filepath.ToSlash(path))
	}
	dataCandidate := filepath.Clean(filepath.Join(p.dataRoot, normalized))
	codeCandidate := filepath.Clean(filepath.Join(p.codeRoot, normalized))
	resolvedData, err := resolvePathWithinRoot(p.dataRoot, dataCandidate)
	if err != nil {
		return pluginFSResolvedPath{}, err
	}
	resolvedCode, err := resolvePathWithinRoot(p.codeRoot, codeCandidate)
	if err != nil {
		return pluginFSResolvedPath{}, err
	}
	relPath := normalized
	if relPath == "" {
		relPath = "."
	}
	return pluginFSResolvedPath{
		RelPath:  relPath,
		DataPath: resolvedData,
		CodePath: resolvedCode,
	}, nil
}

func (p *pluginFS) lookupEntry(resolved pluginFSResolvedPath) (*pluginFSEntry, error) {
	if info, err := os.Stat(resolved.DataPath); err == nil {
		return &pluginFSEntry{Path: resolved.DataPath, Info: info, Layer: "data"}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if info, err := os.Stat(resolved.CodePath); err == nil {
		return &pluginFSEntry{Path: resolved.CodePath, Info: info, Layer: "code"}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return nil, nil
}

func resolvePathWithinRoot(root string, path string) (string, error) {
	path = filepath.Clean(path)
	if !isPathWithinRoot(root, path) {
		return "", fmt.Errorf("path outside plugin root: %s", filepath.ToSlash(path))
	}
	resolved, err := resolveSymlinkSafePath(root, path)
	if err != nil {
		return "", err
	}
	if !isPathWithinRoot(root, resolved) {
		return "", fmt.Errorf("resolved path outside plugin root")
	}
	return resolved, nil
}

func resolveSymlinkSafePath(root string, path string) (string, error) {
	evaluated, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(evaluated), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	ancestor, ancestorErr := evalExistingPath(path)
	if ancestorErr != nil {
		return "", ancestorErr
	}
	if !isPathWithinRoot(root, ancestor) {
		return "", fmt.Errorf("path escapes plugin root via symlink")
	}
	return filepath.Clean(path), nil
}

func evalExistingPath(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		evaluated, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Clean(evaluated), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		current = parent
	}
}

type pluginStorageState struct {
	data       map[string]string
	changed    bool
	totalBytes int64
	limits     pluginStorageLimits
	read       bool
	write      bool
	readOnly   bool
}

type pluginStorageLimits struct {
	MaxKeys       int
	MaxTotalBytes int64
	MaxValueBytes int64
	MaxKeyBytes   int
}

func normalizePluginStorageLimits(limits pluginStorageLimits) pluginStorageLimits {
	if limits.MaxKeys <= 0 {
		limits.MaxKeys = defaultStorageMaxKeys
	}
	if limits.MaxTotalBytes <= 0 {
		limits.MaxTotalBytes = defaultStorageMaxTotalBytes
	}
	if limits.MaxValueBytes <= 0 {
		limits.MaxValueBytes = defaultStorageMaxValueBytes
	}
	if limits.MaxValueBytes > limits.MaxTotalBytes {
		limits.MaxValueBytes = limits.MaxTotalBytes
	}
	if limits.MaxKeyBytes <= 0 {
		limits.MaxKeyBytes = defaultStorageMaxKeyBytes
	}
	return limits
}

func pluginStorageLimitsFromOptions(opts workerOptions) pluginStorageLimits {
	return normalizePluginStorageLimits(pluginStorageLimits{
		MaxKeys:       opts.storageMaxKeys,
		MaxTotalBytes: opts.storageMaxTotalBytes,
		MaxValueBytes: opts.storageMaxValueBytes,
		MaxKeyBytes:   defaultStorageMaxKeyBytes,
	})
}

func pluginStorageEntryBytes(key string, value string) int64 {
	return int64(len(key) + len(value))
}

func newPluginStorageState(snapshot map[string]string, limits pluginStorageLimits) *pluginStorageState {
	limits = normalizePluginStorageLimits(limits)
	state := &pluginStorageState{
		data:   make(map[string]string, len(snapshot)),
		limits: limits,
	}
	for key, value := range snapshot {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		state.data[normalized] = value
		state.totalBytes += pluginStorageEntryBytes(normalized, value)
	}
	return state
}

func (s *pluginStorageState) get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.read = true
	value, ok := s.data[key]
	return value, ok
}

func (s *pluginStorageState) set(key string, value string) bool {
	if s == nil {
		return false
	}
	if s.readOnly {
		return false
	}
	s.write = true
	normalized := strings.TrimSpace(key)
	if normalized == "" {
		return false
	}
	existing, exists := s.data[normalized]
	if exists && existing == value {
		return true
	}
	if s.limits.MaxKeyBytes > 0 && len(normalized) > s.limits.MaxKeyBytes {
		return false
	}
	if s.limits.MaxValueBytes > 0 && int64(len(value)) > s.limits.MaxValueBytes {
		return false
	}
	nextKeys := len(s.data)
	if !exists {
		nextKeys++
	}
	if s.limits.MaxKeys > 0 && nextKeys > s.limits.MaxKeys {
		return false
	}
	existingBytes := int64(0)
	if exists {
		existingBytes = pluginStorageEntryBytes(normalized, existing)
	}
	nextTotalBytes := s.totalBytes - existingBytes + pluginStorageEntryBytes(normalized, value)
	if s.limits.MaxTotalBytes > 0 && nextTotalBytes > s.limits.MaxTotalBytes {
		return false
	}
	s.data[normalized] = value
	s.totalBytes = nextTotalBytes
	s.changed = true
	s.write = true
	return true
}

func (s *pluginStorageState) delete(key string) bool {
	if s == nil {
		return false
	}
	if s.readOnly {
		return false
	}
	s.write = true
	normalized := strings.TrimSpace(key)
	if normalized == "" {
		return false
	}
	if _, exists := s.data[normalized]; !exists {
		return true
	}
	existing := s.data[normalized]
	delete(s.data, normalized)
	s.totalBytes -= pluginStorageEntryBytes(normalized, existing)
	if s.totalBytes < 0 {
		s.totalBytes = 0
	}
	s.changed = true
	s.write = true
	return true
}

func (s *pluginStorageState) list() []string {
	if s == nil {
		return []string{}
	}
	s.read = true
	if len(s.data) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *pluginStorageState) clear() bool {
	if s == nil {
		return false
	}
	if s.readOnly {
		return false
	}
	s.write = true
	if len(s.data) == 0 {
		return true
	}
	s.data = make(map[string]string)
	s.totalBytes = 0
	s.changed = true
	return true
}

func (s *pluginStorageState) snapshot() map[string]string {
	if s == nil || len(s.data) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(s.data))
	for key, value := range s.data {
		out[key] = value
	}
	return out
}

func (s *pluginStorageState) accessMode() string {
	if s == nil {
		return storageAccessNone
	}
	if s.write {
		return storageAccessWrite
	}
	if s.read {
		return storageAccessRead
	}
	return storageAccessNone
}

func normalizeScriptPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !filepath.IsAbs(trimmed) {
		abs, err := filepath.Abs(trimmed)
		if err == nil {
			trimmed = abs
		}
	}
	return filepath.Clean(trimmed)
}

func reqTimeout(req pluginipc.Request, opts workerOptions) time.Duration {
	timeout := opts.timeoutMs
	if req.Sandbox.TimeoutMs > 0 {
		timeout = req.Sandbox.TimeoutMs
	}
	if timeout <= 0 {
		timeout = 30000
	}
	return time.Duration(timeout) * time.Millisecond
}

func exportGojaValue(value goja.Value) interface{} {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return map[string]interface{}{}
	}
	return value.Export()
}

func buildExecutionResponse(
	result goja.Value,
	storageSnapshot map[string]string,
	storageChanged bool,
	metadata map[string]string,
	isFinal bool,
	workspaceState *pluginWorkspaceState,
) pluginipc.Response {
	_, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	workspaceEntries, workspaceCleared := workspaceState.flushDelta()
	return pluginipc.Response{
		Success:          success,
		Healthy:          true,
		Version:          workerVersion,
		Data:             responseData,
		Storage:          storageSnapshot,
		StorageChanged:   storageChanged,
		Error:            errMsg,
		Metadata:         metadata,
		WorkspaceEntries: workspaceEntries,
		WorkspaceCleared: workspaceCleared,
		IsFinal:          isFinal,
	}
}

func buildExecutionErrorResponse(
	err error,
	storageSnapshot map[string]string,
	storageChanged bool,
	metadata map[string]string,
	isFinal bool,
	workspaceState *pluginWorkspaceState,
) pluginipc.Response {
	workspaceEntries, workspaceCleared := workspaceState.flushDelta()
	errMessage := ""
	if err != nil {
		errMessage = err.Error()
	}
	return pluginipc.Response{
		Success:          false,
		Healthy:          true,
		Version:          workerVersion,
		Storage:          storageSnapshot,
		StorageChanged:   storageChanged,
		Error:            errMessage,
		Metadata:         metadata,
		WorkspaceEntries: workspaceEntries,
		WorkspaceCleared: workspaceCleared,
		IsFinal:          isFinal,
	}
}

func buildJSWorkerExecutionMetadata(base map[string]string, storageAccessMode string) map[string]string {
	metadata := mergeStringMaps(base, nil)
	if metadata == nil {
		metadata = make(map[string]string, 1)
	}
	metadata[storageAccessMetaKey] = normalizeWorkerStorageAccessMode(storageAccessMode)
	return metadata
}

func normalizeWorkerStorageAccessMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case storageAccessRead:
		return storageAccessRead
	case storageAccessWrite:
		return storageAccessWrite
	default:
		return storageAccessNone
	}
}

func parseExecutionPayload(exported interface{}) (map[string]interface{}, map[string]interface{}, bool, string) {
	payload := normalizePluginStreamChunkData(exported)
	responseData := payload
	if dataValue, exists := payload["data"]; exists {
		responseData = normalizePluginStreamChunkData(dataValue)
	}

	success := true
	if value, exists := payload["success"]; exists {
		if parsed, ok := interfaceToBool(value); ok {
			success = parsed
		}
	}
	errMsg := ""
	if !success {
		if value, ok := payload["error"].(string); ok {
			errMsg = strings.TrimSpace(value)
		}
		if errMsg == "" {
			errMsg = "plugin execution returned success=false"
		}
	}
	return payload, responseData, success, errMsg
}

func normalizePluginStreamChunkData(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case nil:
		return map[string]interface{}{}
	case map[string]interface{}:
		return typed
	case map[string]string:
		out := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out
	default:
		return map[string]interface{}{"value": typed}
	}
}

func normalizePluginStreamProgressValue(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return typed, true
	case uint:
		return typed, true
	case uint8:
		return uint(typed), true
	case uint16:
		return uint(typed), true
	case uint32:
		return uint(typed), true
	case uint64:
		return typed, true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil, false
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed, true
		}
	}
	return nil, false
}

func convertMetadata(input map[string]interface{}, base map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(input))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range input {
		out[key] = fmt.Sprintf("%v", value)
	}
	return out
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func interfaceToBool(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "1", "true", "yes", "y":
			return true, true
		case "0", "false", "no", "n":
			return false, true
		}
	}
	return false, false
}

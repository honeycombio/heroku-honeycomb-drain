# heroku-honeycomb-drain

[![OSS Lifecycle](https://img.shields.io/osslifecycle/honeycombio/REPO)](https://github.com/honeycombio/home/blob/main/honeycomb-oss-lifecycle-and-practices.md)

This is a [log drain](https://devcenter.heroku.com/articles/log-drains) that can be added to any app running on Heroku to start observing that app's behaviour using [Honeycomb](https://honeycomb.io/).

It can send HTTP request events from the Heroku router, along with other events emitted from the Heroku platform. With some configuration it can also parse and send structured logs emitted by app code.

## Getting started

You need to set up your own instance of the log drain to receive logs from your app and send them to Honeycomb. The easiest way to do so is to deploy the drain to Heroku as well (as a separate app).

Clicking the following button will launch a Heroku UI that will deploy this drain as a new app in your Heroku account, prompting you along the way for configuration options.

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/dreid/heroku-honeycomb-drain)

### Configuring the drain

Config is specified via environment variables (if the drain is deployed on Heroku, `heroku config`).

The drain requires the following config params:

 * `HONEYCOMB_WRITE_KEY`: the write key that identifies your Honeycomb team and enables you to send events. You can find this on your Honeycomb [Account page](https://ui.honeycomb.io/account).
 * `ALLOWED_APPS`: authentication credentials for this drain, e.g. `myapp:secretpassword`. See [Authentication](#authentication).
 * `APP_FORMATS`: specifies how to identify and parse the various types of logs emitted by your app, e.g. `heroku/*:logfmt,app/*:raw`. See [Parsing logs](#parsing-logs).

The drain also supports the following optional config params:

 * `HONEYCOMB_DATASET`: set this to control which Honeycomb dataset this drain will send events to, e.g. `myapp` or `heroku`.

### Configuring your app to send logs to the drain

1. Once your instance of the drain is deployed, grab its url (e.g. `https://my-log-drain.herokuapp.com`).
2. Add whatever [credentials](#authentication) you configured in `ALLOWED_APPS` to the URL as HTTP Basic Authentication so that your app can authenticate to the drain (e.g. `https://myapp:secretpassword@my-log-drain.herokuapp.com`).
3. `heroku drains:add https://myapp:secretpassword@my-log-drain.herokuapp.com --app=myapp`

### Authentication

The drain uses HTTP Basic Authentication to ensure only your apps can send events to this log drain (and therefore to your Honeycomb dataset).

The authentication credentials are configured via the `ALLOWED_APPS` config param. The syntax of `ALLOWED_APPS` is a comma-delimited list of `app:password` pairs, e.g. `myapp1:password1,myapp2:password2`.

### Parsing logs

Heroku's logging infrastructure treats logs as unstructured strings, with a few bits of metadata to identify the source of each log entry (e.g. to distinguish logs emitted by app code from those emitted by the Heroku platform itself). Honeycomb works best with structured logs, so this drain supports configuring parsers for each log source to transform logs into structured data.

Log parsers are configured via the `APP_FORMATS` config param. The syntax of `APP_FORMATS` is a comma-delimited list of `pattern:parser` pairs, defined as follows.

Patterns are matched against the log source (`proc_id` field). Patterns can either be literal (e.g. `heroku/router`), or partial wildcards (e.g. `app/*` to match logs from any app process type). Wildcards are only supported after the slash (e.g. `*/*` is not supported).

Parsers are identified by name, and must be one of the following:

 * `logfmt`: parses the [logfmt](https://brandur.org/logfmt) "key1=value1 key2=value2" format (as used by the Heroku platform logs).
 * `json`: parses JSON logs.
 * `raw`: passes through unstructured text logs.
 * `ignore`: ignores the matched logs.

If you are not sure what to set `APP_FORMATS` to, start with `heroku/*:logfmt,app/*:raw`. That will get you structured logs for things like HTTP requests, dyno CPU and RAM usage, database health metrics, and unstructured logs emitted from your app.

For the best experience, try configuring your app to emit logs as structured JSON, then replace `app/*:raw` with `app/*:json` in `APP_FORMATS`.

// pages.systemSiteSettings.* — System/SiteSettings tabs
export default {
  'pages.systemSiteSettings.action.reset': 'Restore',
  'pages.systemSiteSettings.action.save': 'Save',
  'pages.systemSiteSettings.auth.action.save': 'Save',
  'pages.systemSiteSettings.auth.bindDnLabel': 'Bind DN (read-only account)',
  'pages.systemSiteSettings.auth.bindPasswordLabel': 'Bind Password',
  'pages.systemSiteSettings.auth.callbackUrlLabel': 'Callback URL',
  'pages.systemSiteSettings.auth.callbackUrlTooltip':
    'Callback URL registered on the identity provider: https://<host>/api/v1/auth/oidc/callback',
  'pages.systemSiteSettings.auth.callbackUrlTooltip.github':
    'Callback URL registered on the GitHub OAuth App: https://<host>/api/v1/auth/github/callback',
  'pages.systemSiteSettings.auth.connectionFailed.ldap': 'LDAP connection failed',
  'pages.systemSiteSettings.auth.connectionFailed.oidc': 'OIDC connection failed',
  'pages.systemSiteSettings.auth.dirAddrLabel': 'Directory Address',
  'pages.systemSiteSettings.auth.dirAddrRule': 'ldap://host:389 or ldaps://host:636',
  'pages.systemSiteSettings.auth.enableLdapLabel': 'Enable LDAP Login',
  'pages.systemSiteSettings.auth.enableGitHubLabel': 'Enable GitHub Login',
  'pages.systemSiteSettings.auth.enableSsoLabel': 'Enable SSO Login',
  'pages.systemSiteSettings.auth.error.loadFailed': 'Failed to load authentication settings',
  'pages.systemSiteSettings.auth.error.saveFailed': 'Save failed',
  'pages.systemSiteSettings.auth.error.testFailed': 'Test failed',
  'pages.systemSiteSettings.auth.github.title': 'GitHub OAuth',
  'pages.systemSiteSettings.auth.hint.githubCard':
    'Once enabled, a "GitHub Login" entry appears on the login page; accounts are provisioned automatically on first login',
  'pages.systemSiteSettings.auth.hint.ldapCard':
    'Enter username/password in the standard login form; failed local validation cascades to LDAP automatically',
  'pages.systemSiteSettings.auth.hint.localCard':
    'Disabling hides the password form on the login page; keep at least one login method enabled',
  'pages.systemSiteSettings.auth.hint.oidcCard':
    'Once enabled, an "SSO Login" entry appears on the login page',
  'pages.systemSiteSettings.auth.hint.registerCard':
    'Off by default; once enabled, a "Create account" entry appears on the login page for local password accounts',
  'pages.systemSiteSettings.auth.hint.tab':
    'The config file only provides initial values; changes saved here take effect immediately (no restart needed). Make sure another login method works before disabling password login',
  'pages.systemSiteSettings.auth.local.enableLabel': 'Enable Password Login',
  'pages.systemSiteSettings.auth.local.enableTooltip':
    'Built-in admin accounts use this channel; ensure LDAP/OIDC/GitHub is available first, otherwise the save is rejected to prevent lockout',
  'pages.systemSiteSettings.auth.local.title': 'Local Password',
  'pages.systemSiteSettings.auth.issuerRule': 'e.g. https://sso.example.com',
  'pages.systemSiteSettings.auth.jitRolesLabel': 'JIT Roles',
  'pages.systemSiteSettings.auth.jitRolesTooltip':
    'Roles granted when the account is created automatically on first login (comma-separated)',
  'pages.systemSiteSettings.auth.ldap.title': 'LDAP Directory',
  'pages.systemSiteSettings.auth.oidc.title': 'OIDC Single Sign-On',
  'pages.systemSiteSettings.auth.register.enableLabel': 'Allow self-registration',
  'pages.systemSiteSettings.auth.register.enableTooltip':
    'Anyone can register a local account on the login page; passwords are validated by the account security policy',
  'pages.systemSiteSettings.auth.register.rolesTooltip':
    'Roles granted to registered accounts (comma-separated); leave empty to grant none',
  'pages.systemSiteSettings.auth.register.domainWhitelistLabel': 'Email domain whitelist',
  'pages.systemSiteSettings.auth.register.domainWhitelistTooltip':
    'Comma-separated domain suffixes; subdomains allowed automatically (example.com covers api.example.com); empty = unrestricted',
  'pages.systemSiteSettings.auth.register.aliasRestrictionLabel': 'Email alias restriction',
  'pages.systemSiteSettings.auth.register.aliasRestrictionTooltip':
    'Rejects plus aliases (user+tag@) and deduplicates ignoring dots (u.s@ equals us@) when enabled',
  'pages.systemSiteSettings.auth.register.verificationRequiredLabel': 'Email verification',
  'pages.systemSiteSettings.auth.register.verificationRequiredTooltip':
    'When on, sign-up requires an email address verified via mail before login. Users will not receive mail while SMTP is unconfigured (they may retry with username + email).',
  'pages.systemSiteSettings.auth.register.title': 'Self-Registration',
  'pages.systemSiteSettings.auth.provider.disabled': 'Disabled',
  'pages.systemSiteSettings.auth.provider.enabled': 'Enabled',
  'pages.systemSiteSettings.auth.saveAndTest.ldap': 'Save and Test Connection',
  'pages.systemSiteSettings.auth.saveAndTest.oidc': 'Save and Test Discovery Endpoint',
  'pages.systemSiteSettings.auth.saved.ldap': 'LDAP configuration saved',
  'pages.systemSiteSettings.auth.saved.oidc': 'OIDC configuration saved',
  'pages.systemSiteSettings.auth.saved.register': 'Self-registration configuration saved',
  'pages.systemSiteSettings.auth.secretPlaceholderKeep': 'Leave blank to keep unchanged',
  'pages.systemSiteSettings.auth.secretPlaceholderUnset': 'Not set',
  'pages.systemSiteSettings.auth.secretSavedTooltip':
    'Saved: {masked}; leave blank to keep unchanged',
  'pages.systemSiteSettings.auth.sourceTag.configFile': 'Config File',
  'pages.systemSiteSettings.auth.sourceTag.default': 'Default',
  'pages.systemSiteSettings.auth.sourceTag.ui': 'UI',
  'pages.systemSiteSettings.auth.switch.disable': 'Disable',
  'pages.systemSiteSettings.auth.switch.enable': 'Enable',
  'pages.systemSiteSettings.auth.userFilterLabel': 'User Filter',
  'pages.systemSiteSettings.auth.userFilterTooltip':
    "The '{'username'}' placeholder is replaced with the login input; leave empty to use userDnTemplate (config file)",
  'pages.systemSiteSettings.error.loadFailed': 'Failed to load site settings',
  'pages.systemSiteSettings.error.operationFailed': 'Operation failed',
  'pages.systemSiteSettings.error.saveFailed': 'Save failed',
  'pages.systemSiteSettings.featureFlags.column.description': 'Description',
  'pages.systemSiteSettings.featureFlags.column.domain': 'Feature Domain',
  'pages.systemSiteSettings.featureFlags.column.source': 'Source',
  'pages.systemSiteSettings.featureFlags.column.status': 'Status',
  'pages.systemSiteSettings.featureFlags.domain.analytics.description':
    'Real-time dashboards, retention, behavior, and payment analytics',
  'pages.systemSiteSettings.featureFlags.domain.analytics.label': 'Data Analytics',
  'pages.systemSiteSettings.featureFlags.domain.dev.description':
    'Bug tracking, tools, releases, and hotpatch management',
  'pages.systemSiteSettings.featureFlags.domain.dev.label': 'Development',
  'pages.systemSiteSettings.featureFlags.domain.extensions.description':
    'Extension store, installation, and agent sync',
  'pages.systemSiteSettings.featureFlags.domain.extensions.label': 'Extensions',
  'pages.systemSiteSettings.featureFlags.domain.ops.description':
    'Nodes, jobs, alerts, rate limiting, backups, certificates, and DB monitoring',
  'pages.systemSiteSettings.featureFlags.domain.ops.label': 'Operations',
  'pages.systemSiteSettings.featureFlags.domain.support.description':
    'Tickets, FAQ, feedback, and player-facing support entry',
  'pages.systemSiteSettings.featureFlags.domain.support.label': 'Support',
  'pages.systemSiteSettings.featureFlags.error.loadFailed': 'Failed to load feature flags',
  'pages.systemSiteSettings.featureFlags.error.operationFailed': 'Operation failed',
  'pages.systemSiteSettings.featureFlags.hint':
    'Runtime soft switches: changes take effect immediately after saving (menus and APIs are hidden accordingly), no restart required. "Trimmed by deployment" means the domain was explicitly disabled via featureFlags in server.yaml — that is a physical trim; edit the config file and restart before enabling it here.',
  'pages.systemSiteSettings.featureFlags.override.clearConfirm': 'Remove database override?',
  'pages.systemSiteSettings.featureFlags.override.clearDescription':
    'This domain will revert to the default state defined in the deployment config file',
  'pages.systemSiteSettings.featureFlags.override.cleared': 'Reverted to deployment configuration',
  'pages.systemSiteSettings.featureFlags.override.restore': 'Restore',
  'pages.systemSiteSettings.featureFlags.source.dbOverride': 'Database Override',
  'pages.systemSiteSettings.featureFlags.source.deployConfig': 'Following Deployment Config',
  'pages.systemSiteSettings.featureFlags.source.trimmed':
    'Trimmed by Deployment (restart required)',
  'pages.systemSiteSettings.featureFlags.toggle.off':
    'Disabled; related menus and APIs are now hidden',
  'pages.systemSiteSettings.featureFlags.toggle.on': 'Enabled; menus update immediately',
  'pages.systemSiteSettings.field.copyright': 'Footer Copyright',
  'pages.systemSiteSettings.field.description': 'Login Page Subtitle',
  'pages.systemSiteSettings.field.serverUrl': 'Server Address',
  'pages.systemSiteSettings.field.serverUrlPlaceholder': 'https://gm.example.com (public domain)',
  'pages.systemSiteSettings.field.taskPublicUrl': 'Async Task Public URL',
  'pages.systemSiteSettings.field.taskPublicUrlPlaceholder': 'https://tasks.example.com',
  'pages.systemSiteSettings.field.docsUrl': 'Docs Link (About)',
  'pages.systemSiteSettings.field.docsUrlPlaceholder': 'https://docs.example.com',
  'pages.systemSiteSettings.field.homeContent': 'Home Content',
  'pages.systemSiteSettings.field.homeContentPlaceholder':
    'Intro text shown on the login page welcome area',
  'pages.systemSiteSettings.field.userAgreement': 'User Agreement',
  'pages.systemSiteSettings.field.privacyPolicy': 'Privacy Policy',
  'pages.systemSiteSettings.field.agreementPlaceholder':
    'Full agreement text, shown on the login page footer',
  'pages.systemSiteSettings.field.faviconUrl': 'Favicon URL',
  'pages.systemSiteSettings.field.icp': 'ICP Filing Number',
  'pages.systemSiteSettings.field.icpPlaceholder': 'Beijing ICP Filing No. XXXXXXXX',
  'pages.systemSiteSettings.field.logoUrl': 'Logo URL',
  'pages.systemSiteSettings.field.logoUrlPlaceholder': '/logo.svg or https://…',
  'pages.systemSiteSettings.field.siteName': 'Site Name',
  'pages.systemSiteSettings.hint':
    'Configuration layers: code defaults ← config file ← override here (highest). The "Restore" button removes the override and falls back to the config file value. Changes take effect immediately, no restart needed.',
  'pages.systemSiteSettings.notification.action.save': 'Save',
  'pages.systemSiteSettings.notification.channel.dingtalk': 'DingTalk Group Robot',
  'pages.systemSiteSettings.notification.channel.feishu': 'Feishu Group Robot',
  'pages.systemSiteSettings.notification.channel.webhook': 'Generic Webhook',
  'pages.systemSiteSettings.notification.channel.wecom': 'WeCom Group Robot',
  'pages.systemSiteSettings.notification.error.loadFailed': 'Failed to load notification settings',
  'pages.systemSiteSettings.notification.error.operationFailed': 'Operation failed',
  'pages.systemSiteSettings.notification.error.saveFailed': 'Save failed',
  'pages.systemSiteSettings.notification.field.dingtalkSecretHelp':
    'Required when the robot security setting uses "signing"',
  'pages.systemSiteSettings.notification.field.dingtalkSecretLabel': 'Signing Secret (SEC…)',
  'pages.systemSiteSettings.notification.field.dingtalkUrlHelp':
    'DingTalk group → Group Settings → Robots → Add a "Custom" robot',
  'pages.systemSiteSettings.notification.field.feishuSecretHelp':
    'Required when the robot security setting enables "signature verification"',
  'pages.systemSiteSettings.notification.field.feishuSecretLabel': 'Signing Secret',
  'pages.systemSiteSettings.notification.field.feishuSecretPlaceholder':
    'Signature verification secret',
  'pages.systemSiteSettings.notification.field.feishuUrlHelp':
    'Feishu group → Settings → Group Robots → Add a "Custom Robot"',
  'pages.systemSiteSettings.notification.field.fromAddressLabel': 'From Address',
  'pages.systemSiteSettings.notification.field.groupWebhookLabel': 'Group Robot Webhook',
  'pages.systemSiteSettings.notification.field.smtpHostLabel': 'SMTP Server',
  'pages.systemSiteSettings.notification.field.smtpPasswordLabel': 'SMTP Password',
  'pages.systemSiteSettings.notification.field.smtpPortLabel': 'SMTP Port',
  'pages.systemSiteSettings.notification.field.smtpUserLabel': 'SMTP Username',
  'pages.systemSiteSettings.notification.field.webhookSecretHelp':
    'Request header X-Croupier-Signature: sha256=… (HMAC of the body)',
  'pages.systemSiteSettings.notification.field.webhookSecretLabel': 'Signing Secret',
  'pages.systemSiteSettings.notification.field.webhookSecretPlaceholder': 'HMAC-SHA256 secret',
  'pages.systemSiteSettings.notification.field.webhookUrlLabel': 'Webhook URL',
  'pages.systemSiteSettings.notification.field.wecomUrlHelp':
    'WeCom group → Group Settings → Group Robots → Add Robot (the key is carried in the URL, no signing)',
  'pages.systemSiteSettings.notification.hint':
    'Notification channels for approval and alert events. In-app messaging is enabled by default (zero config); configure DingTalk / generic Webhook / email as needed — changes take effect on save. Secrets only echo the last 4 characters; saving an empty value clears the override.',
  'pages.systemSiteSettings.notification.saved': 'Saved',
  'pages.systemSiteSettings.notification.secret.configured': 'Configured {masked}',
  'pages.systemSiteSettings.notification.secret.unconfigured': 'Not Configured',
  'pages.systemSiteSettings.notification.toggle.email': 'Email',
  'pages.systemSiteSettings.notification.toggle.inApp': 'In-App',
  'pages.systemSiteSettings.notification.toggle.off': 'Disabled',
  'pages.systemSiteSettings.notification.toggle.on': 'Enabled',
  'pages.systemSiteSettings.observability.action.restore': 'Restore',
  'pages.systemSiteSettings.observability.action.save': 'Save',
  'pages.systemSiteSettings.observability.error.loadFailed':
    'Failed to load observability settings',
  'pages.systemSiteSettings.observability.error.saveFailed': 'Save failed',
  'pages.systemSiteSettings.observability.field.alertmanagerUrlHelp':
    'Alertmanager source used by the Operations Center alert page redirect',
  'pages.systemSiteSettings.observability.field.alertmanagerUrlLabel': 'Alertmanager URL',
  'pages.systemSiteSettings.observability.field.grafanaExploreUrlHelp':
    'Grafana entry used for metric drill-downs',
  'pages.systemSiteSettings.observability.field.grafanaExploreUrlLabel': 'Grafana Explore URL',
  'pages.systemSiteSettings.observability.field.jaegerUrlHelp': 'Trace lookup entry',
  'pages.systemSiteSettings.observability.field.jaegerUrlLabel': 'Jaeger URL',
  'pages.systemSiteSettings.observability.hint':
    'Observability platform integration endpoints: once configured, the alert/metrics/tracing pages in the Operations Center use these URLs for redirects. Values stored in the database survive restarts; clear a field and save to revert to the environment variable default.',
  'pages.systemSiteSettings.observability.resetTooltip':
    'Remove the database override and restore the environment variable/default value',
  'pages.systemSiteSettings.observability.saved': 'Saved and effective immediately',
  'pages.systemSiteSettings.observability.source.dbOverride': 'Database Override',
  'pages.systemSiteSettings.observability.source.envVar': 'Environment Variable',
  'pages.systemSiteSettings.observability.source.unconfigured': 'Not Configured',
  'pages.systemSiteSettings.resetSuccess': 'Reverted to config file',
  'pages.systemSiteSettings.resetTooltip':
    'Remove the database override and restore the value from the config file',
  'pages.systemSiteSettings.saved': 'Saved and effective immediately',
  'pages.systemSiteSettings.source.configFile': 'Following Config File',
  'pages.systemSiteSettings.source.dbOverride': 'Database Override',
  'pages.systemSiteSettings.source.default': 'Default',
  'pages.systemSiteSettings.tab.auth': 'Authentication',
  'pages.systemSiteSettings.tab.features': 'Feature Flags',
  'pages.systemSiteSettings.tab.notification': 'Notifications',
  'pages.systemSiteSettings.tab.observability': 'Observability',
  'pages.systemSiteSettings.security.error.loadFailed': 'Failed to load account security policy',
  'pages.systemSiteSettings.security.error.saveFailed': 'Failed to save',
  'pages.systemSiteSettings.security.intro':
    'Everything defaults to off: when off, the built-in baseline applies (password 8-128 chars, weak-password block, two character classes, no expiry, optional TOTP).',
  'pages.systemSiteSettings.security.mfaRequired': 'Require two-factor auth (TOTP)',
  'pages.systemSiteSettings.security.mfaRequiredHelp':
    'When on, every local account must bind TOTP: unbound accounts get a setup prompt at login and other APIs return 403 mfa_required (external IdP accounts are unaffected)',
  'pages.systemSiteSettings.security.maxAgeDays': 'Password max age (days)',
  'pages.systemSiteSettings.security.maxAgeDaysHelp':
    '0 = never expires; when on, password changes/creations start the clock and expired logins force a password change',
  'pages.systemSiteSettings.security.minLength': 'Password minimum length',
  'pages.systemSiteSettings.security.minLengthHelp':
    '0 = built-in baseline of 8 (max 128); can only tighten, never loosen',
  'pages.systemSiteSettings.security.requireSpecial': 'Require special character in passwords',
  'pages.systemSiteSettings.security.requireSpecialHelp':
    'Applies to account creation, password reset, and self-service change',
  'pages.systemSiteSettings.security.requireUppercase': 'Require uppercase letter in passwords',
  'pages.systemSiteSettings.security.requireUppercaseHelp':
    'Applies to account creation, password reset, and self-service change',
  'pages.systemSiteSettings.security.save': 'Save',
  'pages.systemSiteSettings.security.saved': 'Saved',
  'pages.systemSiteSettings.security.cardTitle': 'Account Security Policy',
  'pages.systemSiteSettings.tab.security': 'Account Security',

  // Outbound security & restrictions (OPEN-ISSUES #56)
  'pages.systemSiteSettings.outbound.title': 'Outbound Security & Restrictions',
  'pages.systemSiteSettings.outbound.hint':
    'Restrict outbound HTTP requests made by the platform (notification webhooks, update checks). All off by default: with everything off, outbound behavior is unchanged.',
  'pages.systemSiteSettings.outbound.allowPorts': 'Allowed ports',
  'pages.systemSiteSettings.outbound.allowPortsHelp':
    'Comma-separated port list (1-65535), e.g. 443,8080; empty = unrestricted',
  'pages.systemSiteSettings.outbound.allowIPs': 'Allowed private IPs',
  'pages.systemSiteSettings.outbound.allowIPsHelp':
    'Allowlist for targets blocked by SSRF protection (single IP or CIDR, e.g. 10.0.0.0/8); empty = none',
  'pages.systemSiteSettings.outbound.domainFilter': 'Domain filter (allowlist)',
  'pages.systemSiteSettings.outbound.domainFilterHelp':
    'Comma-separated domain suffixes; subdomains are allowed automatically (example.com covers api.example.com); empty = unrestricted',
  'pages.systemSiteSettings.outbound.ssrfProtection': 'SSRF protection',
  'pages.systemSiteSettings.outbound.ssrfProtectionHelp':
    'When on, outbound targets resolving or connecting to private/loopback/link-local addresses are rejected (use "Allowed private IPs" to permit internal dependencies); the real connection is re-checked before dialing, so DNS rebinding is ineffective.',
  'pages.systemSiteSettings.outbound.boundary':
    'The guard only covers user-configurable outbound HTTP (notification webhooks and update checks); agent / database / SDK channels are not restricted. Domain filter is allowlist semantics (once set, only listed domains can be reached).',
  'pages.systemSiteSettings.outbound.save': 'Save',
  'pages.systemSiteSettings.outbound.saved': 'Saved',
  'pages.systemSiteSettings.outbound.error.loadFailed': 'Failed to load outbound security settings',
  'pages.systemSiteSettings.outbound.error.saveFailed': 'Save failed',

  'pages.systemSiteSettings.tab.site': 'System Info',

  // 运维/系统维护（OPEN-ISSUES #52）
  'pages.systemSiteSettings.maintenance.buildTime': 'Build Time',
  'pages.systemSiteSettings.maintenance.checkNow': 'Check for Updates',
  'pages.systemSiteSettings.maintenance.docs': 'Docs (About)',
  'pages.systemSiteSettings.maintenance.error.checkFailed': 'Update check failed',
  'pages.systemSiteSettings.maintenance.error.loadFailed': 'Failed to load runtime info',
  'pages.systemSiteSettings.maintenance.gitCommit': 'Git Commit',
  'pages.systemSiteSettings.maintenance.runtime': 'Runtime',
  'pages.systemSiteSettings.maintenance.startedAt': 'Started At',
  'pages.systemSiteSettings.maintenance.upToDate': 'Version check result',
  'pages.systemSiteSettings.maintenance.update': 'Check for Updates',
  'pages.systemSiteSettings.maintenance.updateAvailable': 'New version available',
  'pages.systemSiteSettings.maintenance.updateHint':
    'Checks and shows a version note only; upgrades are never executed automatically. Without an update source configured, only the current version is shown.',
  'pages.systemSiteSettings.maintenance.uptime': 'Uptime',
  'pages.systemSiteSettings.maintenance.version': 'Version',

  // Maintenance / third-party service health probe (OPEN-ISSUES #57)
  'pages.systemSiteSettings.maintenance.probe.title': 'Third-Party Service Health Probe',
  'pages.systemSiteSettings.maintenance.probe.hint':
    'Manually probe configured third-party targets: webhook channels use read-only GET (no notification is sent), SMTP uses TCP+EHLO (no mail, no auth).',
  'pages.systemSiteSettings.maintenance.probe.channel.dingtalk': 'DingTalk Bot',
  'pages.systemSiteSettings.maintenance.probe.channel.wecom': 'WeCom Bot',
  'pages.systemSiteSettings.maintenance.probe.channel.feishu': 'Feishu Bot',
  'pages.systemSiteSettings.maintenance.probe.channel.webhook': 'Generic Webhook',
  'pages.systemSiteSettings.maintenance.probe.channel.update': 'Update Source',
  'pages.systemSiteSettings.maintenance.probe.channel.smtp': 'SMTP Mail Service',
  'pages.systemSiteSettings.maintenance.probe.action': 'Probe',
  'pages.systemSiteSettings.maintenance.probe.healthy': 'Healthy',
  'pages.systemSiteSettings.maintenance.probe.unhealthy': 'Unhealthy',
  'pages.systemSiteSettings.maintenance.probe.unconfigured': 'Not configured',
  'pages.systemSiteSettings.maintenance.probe.error.failed': 'Probe failed',
  'pages.systemSiteSettings.tab.maintenance': 'Maintenance',
  'pages.systemSiteSettings.tab.performance': 'Performance',
  'pages.systemSiteSettings.performance.action.refresh': 'Refresh',
  'pages.systemSiteSettings.performance.action.save': 'Save',
  'pages.systemSiteSettings.performance.boundary':
    'Thresholds are annotations only (over-limit items are flagged red here); requests are not blocked in this batch. GOMAXPROCS and cache capacity are read-only displays',
  'pages.systemSiteSettings.performance.error.load': 'Failed to load performance data',
  'pages.systemSiteSettings.performance.error.save': 'Save failed',
  'pages.systemSiteSettings.performance.field.cacheSizeMb': 'Memory Cache (MB)',
  'pages.systemSiteSettings.performance.field.disk': 'Disk',
  'pages.systemSiteSettings.performance.field.maxConcurrent': 'Max Concurrent Requests',
  'pages.systemSiteSettings.performance.field.maxCpuPct': 'CPU Threshold %',
  'pages.systemSiteSettings.performance.field.maxDiskPct': 'Disk Threshold %',
  'pages.systemSiteSettings.performance.field.maxMemoryPct': 'Memory Threshold %',
  'pages.systemSiteSettings.performance.field.maxThreadCount': 'Max Thread Count',
  'pages.systemSiteSettings.performance.field.memory': 'Memory',
  'pages.systemSiteSettings.performance.overlimit': 'Over limit',
  'pages.systemSiteSettings.performance.runtime.diskPath': 'Sampled Path',
  'pages.systemSiteSettings.performance.runtime.gc': 'GC Count / Total Pause',
  'pages.systemSiteSettings.performance.runtime.heapAlloc': 'Heap (Live)',
  'pages.systemSiteSettings.performance.runtime.host': 'Host',
  'pages.systemSiteSettings.performance.runtime.process': 'Process',
  'pages.systemSiteSettings.performance.runtime.sysBytes': 'Process Memory',
  'pages.systemSiteSettings.performance.runtime.title': 'Runtime Stats',
  'pages.systemSiteSettings.performance.runtime.uptime': 'Uptime',
  'pages.systemSiteSettings.performance.saved': 'Performance settings saved',
  'pages.systemSiteSettings.performance.settings.title': 'Performance Settings',
  'pages.systemSiteSettings.performance.tooltip.cache':
    'Stored only in this batch; not wired to the runtime cache yet',
  'pages.systemSiteSettings.performance.tooltip.thread':
    'Recorded only; GOMAXPROCS is not hot-changed',
  'pages.systemSiteSettings.performance.tooltip.zeroOff': '0 = this limit is disabled',
  'pages.systemSiteSettings.tab.logs': 'Logs',
  'pages.systemSiteSettings.logs.action.refresh': 'Refresh',
  'pages.systemSiteSettings.logs.action.save': 'Save',
  'pages.systemSiteSettings.logs.boundary':
    'Cleanup runs hourly; server log rotation is config-file level (restart required) and shown read-only here; log.cleanupCron / copierDir / copierKeep are unwired placeholders',
  'pages.systemSiteSettings.logs.cleanup.action': 'Clean up',
  'pages.systemSiteSettings.logs.cleanup.cancel': 'Cancel',
  'pages.systemSiteSettings.logs.cleanup.confirmOk': 'Confirm cleanup',
  'pages.systemSiteSettings.logs.cleanup.confirmTitle':
    'Clean up audit trail logs older than the selected time? This cannot be undone',
  'pages.systemSiteSettings.logs.cleanup.done':
    'Cleaned: {exec} execution logs, {runs} task runs, {events} task events',
  'pages.systemSiteSettings.logs.cleanup.failed': 'Cleanup failed',
  'pages.systemSiteSettings.logs.cleanup.hint':
    'Clean up trail logs by time; this cannot be undone; audit_records (hash-chain audit) are never cleaned',
  'pages.systemSiteSettings.logs.cleanup.hours': 'Older than',
  'pages.systemSiteSettings.logs.cleanup.hoursUnit': 'hours',
  'pages.systemSiteSettings.logs.cleanup.preset24h': '24 hours ago',
  'pages.systemSiteSettings.logs.cleanup.preset30d': '30 days ago',
  'pages.systemSiteSettings.logs.cleanup.preset7d': '7 days ago',
  'pages.systemSiteSettings.logs.cleanup.scope': 'Scope',
  'pages.systemSiteSettings.logs.cleanup.scope.all': 'All trails',
  'pages.systemSiteSettings.logs.cleanup.scope.execution': 'Execution logs',
  'pages.systemSiteSettings.logs.cleanup.scope.task': 'Task trails',
  'pages.systemSiteSettings.logs.cleanup.title': 'Manual cleanup',
  'pages.systemSiteSettings.logs.error.load': 'Failed to load log data',
  'pages.systemSiteSettings.logs.error.save': 'Save failed',
  'pages.systemSiteSettings.logs.retention.daysUnit': 'days',
  'pages.systemSiteSettings.logs.retention.effective': 'Currently effective',
  'pages.systemSiteSettings.logs.retention.effectiveExec': 'Execution log retention',
  'pages.systemSiteSettings.logs.retention.effectiveTask': 'Task trail retention',
  'pages.systemSiteSettings.logs.retention.field.days': 'Trail retention days',
  'pages.systemSiteSettings.logs.retention.forever': 'Forever',
  'pages.systemSiteSettings.logs.retention.title': 'Retention policy',
  'pages.systemSiteSettings.logs.retention.tooltip.days':
    'Unified retention for execution and task trails; 0 = follow config file (default 7 days)',
  'pages.systemSiteSettings.logs.saved': 'Log settings saved',
  'pages.systemSiteSettings.logs.server.compress': 'Compress old files',
  'pages.systemSiteSettings.logs.server.directory': 'Log directory',
  'pages.systemSiteSettings.logs.server.file': 'Log file',
  'pages.systemSiteSettings.logs.server.fileCount': 'File count',
  'pages.systemSiteSettings.logs.server.maxAge': 'Max age (days)',
  'pages.systemSiteSettings.logs.server.maxBackups': 'Max backups',
  'pages.systemSiteSettings.logs.server.maxSize': 'Max size (MB)',
  'pages.systemSiteSettings.logs.server.output': 'Output',
  'pages.systemSiteSettings.logs.server.title': 'Server log files',
  'pages.systemSiteSettings.logs.tables.col.oldest': 'Oldest record',
  'pages.systemSiteSettings.logs.tables.col.rows': 'Rows',
  'pages.systemSiteSettings.logs.tables.col.table': 'Table',
  'pages.systemSiteSettings.logs.tables.title': 'Trail table sizes',
  'pages.systemSiteSettings.smtp.title': 'SMTP Email Service',
  'pages.systemSiteSettings.smtp.hint':
    'Transport settings for the notification email channel; encryption left empty = auto (implicit TLS on port 465, STARTTLS upgrade when advertised on other ports). Changes apply immediately.',
  'pages.systemSiteSettings.smtp.encryption': 'Encryption',
  'pages.systemSiteSettings.smtp.encryptionTooltip':
    'Auto: implicit TLS on port 465; STARTTLS upgrade when advertised on other ports',
  'pages.systemSiteSettings.smtp.encryptionAuto': 'Auto',
  'pages.systemSiteSettings.smtp.encryptionNone': 'None (plaintext)',
  'pages.systemSiteSettings.smtp.encryptionSsl': 'SSL/TLS (implicit)',
  'pages.systemSiteSettings.smtp.encryptionStarttls': 'STARTTLS (forced)',
  'pages.systemSiteSettings.smtp.authType': 'Auth method',
  'pages.systemSiteSettings.smtp.authTypeTooltip':
    'Some providers only support AUTH LOGIN; PLAIN is the standard default',
  'pages.systemSiteSettings.smtp.authTypePlain': 'PLAIN (default)',
  'pages.systemSiteSettings.smtp.authTypeLogin': 'AUTH LOGIN (forced)',
  'pages.systemSiteSettings.smtp.skipVerify': 'Skip TLS certificate verification',
  'pages.systemSiteSettings.smtp.skipVerifyHint':
    'Skip verification only for self-signed intranet servers; enabling it for public providers exposes MITM risk.',
  'pages.systemSiteSettings.smtp.testEmailLabel': 'Send test email',
  'pages.systemSiteSettings.smtp.testEmailTooltip':
    'Sends a real email using the saved SMTP config (unsaved form drafts above are ignored)',
  'pages.systemSiteSettings.smtp.testEmailAction': 'Send',
  'pages.systemSiteSettings.smtp.testEmailInvalid': 'Invalid email address',
  'pages.systemSiteSettings.smtp.testEmailSent': 'Test email sent, please check the inbox',
  'pages.systemSiteSettings.smtp.testEmailFailed': 'Send failed',
  'pages.systemSiteSettings.notification.smtpMoved':
    'SMTP server/encryption/auth settings for the email channel have moved to the "SMTP Email Service" card on the Ops tab.',
};

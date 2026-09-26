// pages.profileMfa.* — Profile/MfaSettings (TOTP two-step verification block)
export default {
  'pages.profileMfa.alreadyEnabled': 'Two-step verification is already enabled',
  'pages.profileMfa.cancel.button': 'Cancel',
  'pages.profileMfa.code.placeholder': '6-digit code',
  'pages.profileMfa.confirm.button': 'Confirm and enable',
  'pages.profileMfa.description':
    'Once enabled, sign-in requires a 6-digit dynamic code from an authenticator app (such as Google Authenticator).',
  'pages.profileMfa.disable.button': 'Disable two-step verification',
  'pages.profileMfa.enable.button': 'Enable two-step verification',
  'pages.profileMfa.error.confirm': 'Confirmation failed. Please double-check the code',
  'pages.profileMfa.error.disable': 'Failed to disable. Please double-check the code and password',
  'pages.profileMfa.error.generateSecret': 'Failed to generate the secret',
  'pages.profileMfa.error.loadStatus': 'Failed to load two-step verification status',
  'pages.profileMfa.external.description':
    'This account comes from an external identity source. Two-step verification is managed by the identity provider.',
  'pages.profileMfa.external.tag': 'Managed by IdP',
  'pages.profileMfa.password.placeholder': 'Login password',
  // Backup recovery codes: shown once after enrollment, never again
  'pages.profileMfa.recovery.download': 'Download all recovery codes',
  'pages.profileMfa.recovery.hint':
    'Each recovery code works only once. If you lose your authenticator app, use a recovery code at sign-in instead of the dynamic code. They cannot be viewed again after you dismiss this.',
  'pages.profileMfa.recovery.remaining': 'Recovery codes left {remaining}/{total}',
  'pages.profileMfa.recovery.saved': 'I have saved them, dismiss',
  'pages.profileMfa.recovery.title': 'Save your backup recovery codes now',
  'pages.profileMfa.setup.qrIssuer': 'Issuer: {issuer}',
  'pages.profileMfa.setup.secretWarning':
    'The secret is shown only once. Keep it safe; sign-in will require the dynamic code after confirmation.',
  'pages.profileMfa.setup.step1Bold': 'enter the key manually',
  'pages.profileMfa.setup.step1Prefix': '1. In your authenticator app, ',
  'pages.profileMfa.setup.step1Scan': 'scan the QR code',
  'pages.profileMfa.setup.step1Suffix':
    ' (Microsoft/Google Authenticator, 1Password and others are all supported),',
  'pages.profileMfa.setup.step2Prefix': '2. If you cannot scan, ',
  'pages.profileMfa.setup.step3Bold': '6-digit code',
  'pages.profileMfa.setup.step3Prefix': '3. Enter the ',
  'pages.profileMfa.setup.step3Suffix': ' shown in the app to finish binding',
  'pages.profileMfa.status.disabled': 'Disabled',
  'pages.profileMfa.status.enabled': 'Enabled',
  'pages.profileMfa.success.disabled': 'Two-step verification is now disabled',
  'pages.profileMfa.success.enabled':
    'Two-step verification is enabled. Sign-in will require a verification code next time',
  'pages.profileMfa.title': 'Two-step verification (TOTP)',
  'pages.profileMfa.warning.codeAndPasswordRequired':
    'Please enter the verification code and your login password',
  'pages.profileMfa.warning.codeRequired':
    'Please enter the 6-digit code from your authenticator app',
};

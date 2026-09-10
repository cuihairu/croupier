// pages.profileMfa.* — Profile/MfaSettings (TOTP two-step verification block)
export default {
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
  'pages.profileMfa.setup.secretWarning':
    'The secret is shown only once. Keep it safe; sign-in will require the dynamic code after confirmation.',
  'pages.profileMfa.setup.step1Bold': 'enter the key manually',
  'pages.profileMfa.setup.step1Prefix': '1. In your authenticator app, ',
  'pages.profileMfa.setup.step1Suffix': ', or use ',
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

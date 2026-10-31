document.addEventListener('DOMContentLoaded', function () {
  function showError(message) {
    var element = document.getElementById('auth-error')
    if (element) {
      element.textContent = message
      element.style.display = 'block'
    }
  }

  function hideError() {
    var element = document.getElementById('auth-error')
    if (element) {
      element.textContent = ''
      element.style.display = 'none'
    }
  }

  function errorMessage(body, fallback) {
    return body && body.error && body.error.message ? body.error.message : fallback
  }

  var loginForm = document.getElementById('login-form')
  var mfaForm = document.getElementById('mfa-form')
  var loginLinks = document.getElementById('login-links')
  var emailHint = document.getElementById('mfa-email-hint')
  var codeInput = document.getElementById('mfa-code')
  var resendButton = document.getElementById('mfa-resend')
  var backButton = document.getElementById('mfa-back')
  var resendLabel = resendButton ? resendButton.textContent : 'Resend code'
  var challengeToken = ''
  var cooldownTimer = null

  function stopCooldown() {
    if (cooldownTimer) {
      clearInterval(cooldownTimer)
      cooldownTimer = null
    }
    if (resendButton) {
      resendButton.disabled = false
      resendButton.textContent = resendLabel
    }
  }

  function startCooldown(seconds) {
    if (!resendButton) {
      return
    }

    var remaining = seconds
    resendButton.disabled = true
    resendButton.textContent = resendLabel + ' (' + remaining + 's)'

    cooldownTimer = setInterval(function () {
      remaining -= 1
      if (remaining <= 0) {
        stopCooldown()
        return
      }
      resendButton.textContent = resendLabel + ' (' + remaining + 's)'
    }, 1000)
  }

  function showCredentials() {
    challengeToken = ''
    hideError()
    stopCooldown()
    if (mfaForm) {
      mfaForm.style.display = 'none'
    }
    if (loginForm) {
      loginForm.style.display = ''
      var password = loginForm.querySelector('input[name="password"]')
      if (password) {
        password.value = ''
      }
    }
    if (loginLinks) {
      loginLinks.style.display = ''
    }
  }

  function showCodeStep(hint) {
    hideError()
    if (loginForm) {
      loginForm.style.display = 'none'
    }
    if (loginLinks) {
      loginLinks.style.display = 'none'
    }
    if (mfaForm) {
      mfaForm.style.display = ''
    }
    if (emailHint) {
      emailHint.textContent = hint
    }
    if (codeInput) {
      codeInput.value = ''
      codeInput.focus()
    }
    startCooldown(60)
  }

  if (loginForm) {
    loginForm.addEventListener('submit', function (event) {
      event.preventDefault()
      hideError()

      var submit = loginForm.querySelector('button[type="submit"]')
      if (submit) {
        submit.disabled = true
      }

      fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: this.username.value, password: this.password.value }),
      })
        .then(function (response) {
          return response.json()
        })
        .then(function (body) {
          var data = body && body.data ? body.data : {}

          if (data.firstLoginRequired) {
            window.location.href = '/admin/first-login'
            return
          }

          if (data.mfaRequired && data.challengeToken) {
            challengeToken = data.challengeToken
            showCodeStep(data.emailHint || '')
            return
          }

          showError(errorMessage(body, 'Login failed'))
        })
        .catch(function () {
          showError('Login failed')
        })
        .finally(function () {
          if (submit) {
            submit.disabled = false
          }
        })
    })
  }

  if (mfaForm) {
    mfaForm.addEventListener('submit', function (event) {
      event.preventDefault()
      hideError()

      var submit = document.getElementById('mfa-submit')
      if (submit) {
        submit.disabled = true
      }

      fetch('/api/auth/mfa/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ challengeToken: challengeToken, code: this.code.value }),
      })
        .then(function (response) {
          return response.json()
        })
        .then(function (body) {
          if (body && body.data && body.data.user) {
            window.location.href = '/'
            return
          }

          var deadChallenge =
            body &&
            body.error &&
            ['MFA_CHALLENGE_INVALID', 'MFA_CHALLENGE_EXPIRED', 'MFA_MAX_ATTEMPTS'].indexOf(body.error.code) !== -1

          if (deadChallenge) {
            showCredentials()
          }

          showError(errorMessage(body, 'Verification failed'))
        })
        .catch(function () {
          showError('Verification failed')
        })
        .finally(function () {
          if (submit) {
            submit.disabled = false
          }
        })
    })
  }

  if (resendButton) {
    resendButton.addEventListener('click', function () {
      if (resendButton.disabled || !challengeToken) {
        return
      }

      hideError()
      resendButton.disabled = true

      fetch('/api/auth/mfa/resend', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ challengeToken: challengeToken }),
      })
        .then(function (response) {
          return response.json()
        })
        .then(function (body) {
          if (body && body.data && body.data.challengeToken) {
            challengeToken = body.data.challengeToken
            if (emailHint) {
              emailHint.textContent = body.data.emailHint || ''
            }
            if (codeInput) {
              codeInput.value = ''
              codeInput.focus()
            }
            startCooldown(60)
            return
          }

          if (
            body &&
            body.error &&
            ['MFA_CHALLENGE_INVALID', 'MFA_CHALLENGE_EXPIRED', 'MFA_MAX_ATTEMPTS', 'MFA_UNAVAILABLE'].indexOf(body.error.code) !== -1
          ) {
            showCredentials()
          } else {
            resendButton.disabled = false
          }

          showError(errorMessage(body, 'Could not resend the code'))
        })
        .catch(function () {
          resendButton.disabled = false
          showError('Could not resend the code')
        })
    })
  }

  if (backButton) {
    backButton.addEventListener('click', showCredentials)
  }

  var registerForm = document.getElementById('register-form')
  if (registerForm) {
    registerForm.addEventListener('submit', function (event) {
      event.preventDefault()
      hideError()
      var consent = document.getElementById('register-consent')
      if (consent && !consent.checked) {
        var isID = (document.documentElement.lang || '').toLowerCase().indexOf('id') === 0
        showError(
          isID
            ? 'Setujui Syarat & Ketentuan dan Kebijakan Privasi untuk melanjutkan.'
            : 'Please agree to the Terms of Service and Privacy Policy to continue.',
        )
        return
      }
      fetch('/api/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: this.username.value,
          name: this.name.value,
          email: this.email.value,
          password: this.password.value,
          consentAccepted: consent ? consent.checked : false,
        }),
      })
        .then(function (response) {
          return response.json()
        })
        .then(function (body) {
          // The register endpoint never returns an account ID: a duplicate
          // username or email reads exactly like a success, and the existing
          // account holder is notified by email instead. Any data payload is
          // a success-shaped reply.
          if (body.data) {
            hideError()
            var form = document.getElementById('register-form')
            var success = document.getElementById('auth-success')
            if (form) {
              form.style.display = 'none'
            }
            if (success) {
              success.style.display = 'block'
            }
          } else {
            showError(errorMessage(body, 'Registration failed'))
          }
        })
        .catch(function () {
          showError('Registration failed')
        })
    })
  }

  var forgotForm = document.getElementById('forgot-form')
  if (forgotForm) {
    forgotForm.addEventListener('submit', function (event) {
      event.preventDefault()
      fetch('/api/auth/forgot-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: this.email.value }),
      })
        .then(function () {
          var form = document.getElementById('forgot-form')
          var success = document.getElementById('auth-success')
          if (form) {
            form.style.display = 'none'
          }
          if (success) {
            success.style.display = 'block'
          }
        })
        .catch(function () {
          var form = document.getElementById('forgot-form')
          var success = document.getElementById('auth-success')
          if (form) {
            form.style.display = 'none'
          }
          if (success) {
            success.style.display = 'block'
          }
        })
    })
  }
})

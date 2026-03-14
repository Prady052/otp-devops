import { useState, useRef, useEffect, useCallback } from 'react'
import './App.css'

const STATES = {
  IDLE: 'idle',
  SENDING: 'sending',
  OTP_SENT: 'otp_sent',
  VERIFYING: 'verifying',
  VERIFIED: 'verified',
}

function App() {
  const [appState, setAppState] = useState(STATES.IDLE)
  const [email, setEmail] = useState('')
  const [otpLength, setOtpLength] = useState(6)
  const [otpDigits, setOtpDigits] = useState(() => new Array(6).fill(''))
  const [devOtp, setDevOtp] = useState('')
  const [message, setMessage] = useState({ text: '', type: '' })
  const [countdown, setCountdown] = useState(0)
  const digitRefs = useRef([])

  // Reset OTP digit array when length changes via the select dropdown
  const handleOtpLengthChange = (newLength) => {
    setOtpLength(newLength)
    setOtpDigits(new Array(newLength).fill(''))
  }

  // Countdown timer for resend
  useEffect(() => {
    if (countdown <= 0) return
    const timer = setInterval(() => {
      setCountdown(prev => {
        if (prev <= 1) {
          clearInterval(timer)
          return 0
        }
        return prev - 1
      })
    }, 1000)
    return () => clearInterval(timer)
  }, [countdown])

  const clearMessage = useCallback(() => {
    setTimeout(() => setMessage({ text: '', type: '' }), 5000)
  }, [])

  // ---------- Request OTP ----------
  const handleRequestOTP = async (e) => {
    e.preventDefault()
    if (!email.trim()) {
      setMessage({ text: 'Please enter your email address', type: 'error' })
      clearMessage()
      return
    }

    setAppState(STATES.SENDING)
    setMessage({ text: '', type: '' })

    try {
      const res = await fetch('/api/otp/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ identifier: email, otp_length: otpLength }),
      })
      const data = await res.json()

      if (data.success) {
        setAppState(STATES.OTP_SENT)
        setDevOtp(data.otp || '')
        setOtpDigits(new Array(otpLength).fill(''))
        setCountdown(60)
        setMessage({ text: 'OTP sent successfully! Check your email.', type: 'success' })
        clearMessage()
        // Focus first digit after DOM update
        setTimeout(() => digitRefs.current[0]?.focus(), 100)
      } else {
        setAppState(STATES.IDLE)
        setMessage({ text: data.error || 'Failed to send OTP', type: 'error' })
        clearMessage()
      }
    } catch {
      setAppState(STATES.IDLE)
      setMessage({ text: 'Network error. Is the backend running?', type: 'error' })
      clearMessage()
    }
  }

  // ---------- Verify OTP ----------
  const handleVerifyOTP = async (e) => {
    e.preventDefault()
    const otp = otpDigits.join('')
    if (otp.length !== otpLength) {
      setMessage({ text: `Please enter all ${otpLength} digits`, type: 'error' })
      clearMessage()
      return
    }

    setAppState(STATES.VERIFYING)
    setMessage({ text: '', type: '' })

    try {
      const res = await fetch('/api/otp/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ identifier: email, otp }),
      })
      const data = await res.json()

      if (data.success) {
        setAppState(STATES.VERIFIED)
        setDevOtp('')
        setMessage({ text: 'OTP verified successfully!', type: 'success' })
      } else {
        setAppState(STATES.OTP_SENT)
        setMessage({ text: data.error || 'Invalid OTP', type: 'error' })
        clearMessage()
      }
    } catch {
      setAppState(STATES.OTP_SENT)
      setMessage({ text: 'Network error. Please try again.', type: 'error' })
      clearMessage()
    }
  }

  // ---------- OTP Digit Handling ----------
  const handleDigitChange = (index, value) => {
    // Only accept numeric input
    if (value && !/^\d$/.test(value)) return

    const newDigits = [...otpDigits]
    newDigits[index] = value
    setOtpDigits(newDigits)

    // Auto-focus next input
    if (value && index < otpLength - 1) {
      digitRefs.current[index + 1]?.focus()
    }
  }

  const handleDigitKeyDown = (index, e) => {
    if (e.key === 'Backspace' && !otpDigits[index] && index > 0) {
      digitRefs.current[index - 1]?.focus()
    }
  }

  const handleDigitPaste = (e) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, otpLength)
    if (!pasted) return
    const newDigits = [...otpDigits]
    for (let i = 0; i < pasted.length; i++) {
      newDigits[i] = pasted[i]
    }
    setOtpDigits(newDigits)
    const focusIdx = Math.min(pasted.length, otpLength - 1)
    digitRefs.current[focusIdx]?.focus()
  }

  // ---------- Resend ----------
  const handleResend = async () => {
    if (countdown > 0) return
    setMessage({ text: '', type: '' })
    setAppState(STATES.SENDING)

    try {
      const res = await fetch('/api/otp/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ identifier: email, otp_length: otpLength }),
      })
      const data = await res.json()

      if (data.success) {
        setDevOtp(data.otp || '')
        setOtpDigits(new Array(otpLength).fill(''))
        setCountdown(60)
        setAppState(STATES.OTP_SENT)
        setMessage({ text: 'New OTP sent!', type: 'success' })
        clearMessage()
        setTimeout(() => digitRefs.current[0]?.focus(), 100)
      } else {
        setAppState(STATES.OTP_SENT)
        setMessage({ text: data.error || 'Failed to resend', type: 'error' })
        clearMessage()
      }
    } catch {
      setAppState(STATES.OTP_SENT)
      setMessage({ text: 'Network error', type: 'error' })
      clearMessage()
    }
  }

  // ---------- Reset ----------
  const handleReset = () => {
    setAppState(STATES.IDLE)
    setEmail('')
    setOtpDigits(new Array(otpLength).fill(''))
    setDevOtp('')
    setMessage({ text: '', type: '' })
    setCountdown(0)
  }

  // ========== RENDER ==========
  return (
    <div className="otp-container">
      <div className="otp-card">

        {/* ---- VERIFIED SCREEN ---- */}
        {appState === STATES.VERIFIED ? (
          <div className="success-screen">
            <div className="checkmark">✓</div>
            <h1>Verified!</h1>
            <p style={{ color: 'var(--text-secondary)', marginBottom: 28 }}>
              Your identity has been confirmed successfully.
            </p>
            <button className="btn btn-ghost" onClick={handleReset}>
              Verify Another
            </button>
          </div>
        ) : (
          <>
            {/* ---- HEADER ---- */}
            <div className="otp-header">
              <div className={`otp-icon ${appState === STATES.OTP_SENT ? 'success-icon' : ''}`}>
                {appState === STATES.OTP_SENT || appState === STATES.VERIFYING ? '📬' : '🔐'}
              </div>
              <h1>
                {appState === STATES.OTP_SENT || appState === STATES.VERIFYING
                  ? 'Enter OTP'
                  : 'OTP Verification'}
              </h1>
              <p>
                {appState === STATES.OTP_SENT || appState === STATES.VERIFYING
                  ? `We've sent a ${otpLength}-digit code to your email`
                  : 'Enter your email to receive a one-time password'}
              </p>
            </div>

            {/* ---- MESSAGE ---- */}
            {message.text && (
              <div className={`message message-${message.type}`}>
                <span>{message.type === 'success' ? '✓' : message.type === 'error' ? '✕' : 'ℹ'}</span>
                {message.text}
              </div>
            )}

            {/* ---- REQUEST FORM ---- */}
            {(appState === STATES.IDLE || appState === STATES.SENDING) && (
              <form className="otp-form" onSubmit={handleRequestOTP}>
                <div className="form-group">
                  <label htmlFor="email-input">Email Address</label>
                  <input
                    id="email-input"
                    type="email"
                    className="form-input"
                    placeholder="you@example.com"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    disabled={appState === STATES.SENDING}
                    autoFocus
                    required
                  />
                </div>

                <div className="form-group">
                  <label htmlFor="otp-length">OTP Length</label>
                  <select
                    id="otp-length"
                    className="form-select"
                    value={otpLength}
                    onChange={(e) => handleOtpLengthChange(Number(e.target.value))}
                    disabled={appState === STATES.SENDING}
                  >
                    {[4, 5, 6, 7, 8].map(n => (
                      <option key={n} value={n}>{n} digits</option>
                    ))}
                  </select>
                </div>

                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={appState === STATES.SENDING}
                >
                  {appState === STATES.SENDING ? (
                    <><span className="spinner"></span>Sending...</>
                  ) : (
                    'Send OTP'
                  )}
                </button>
              </form>
            )}

            {/* ---- VERIFY FORM ---- */}
            {(appState === STATES.OTP_SENT || appState === STATES.VERIFYING) && (
              <form className="otp-form" onSubmit={handleVerifyOTP}>
                {/* Email display (disabled) */}
                <div className="email-display">
                  <span className="email-icon">📧</span>
                  <span className="email-text">{email}</span>
                </div>

                {/* Dev OTP display */}
                {devOtp && (
                  <div className="dev-otp">
                    <div className="dev-label">Dev Mode — Your OTP</div>
                    <div className="dev-code">{devOtp}</div>
                  </div>
                )}

                {/* OTP digit inputs */}
                <div className="form-group">
                  <label>Enter OTP</label>
                  <div className="otp-input-group" onPaste={handleDigitPaste}>
                    {otpDigits.map((digit, i) => (
                      <input
                        key={i}
                        ref={el => digitRefs.current[i] = el}
                        type="text"
                        inputMode="numeric"
                        maxLength={1}
                        className={`otp-digit ${digit ? 'filled' : ''}`}
                        value={digit}
                        onChange={(e) => handleDigitChange(i, e.target.value)}
                        onKeyDown={(e) => handleDigitKeyDown(i, e)}
                        disabled={appState === STATES.VERIFYING}
                      />
                    ))}
                  </div>
                </div>

                {/* Countdown */}
                {countdown > 0 && (
                  <div className="countdown">
                    Resend available in <strong>{countdown}s</strong>
                  </div>
                )}

                <button
                  type="submit"
                  className="btn btn-success"
                  disabled={appState === STATES.VERIFYING || otpDigits.join('').length !== otpLength}
                >
                  {appState === STATES.VERIFYING ? (
                    <><span className="spinner"></span>Verifying...</>
                  ) : (
                    'Verify OTP'
                  )}
                </button>

                <div className="otp-footer">
                  <button
                    type="button"
                    className="link-btn"
                    onClick={handleResend}
                    disabled={countdown > 0 || appState === STATES.VERIFYING}
                  >
                    Resend OTP
                  </button>
                  <button
                    type="button"
                    className="link-btn"
                    onClick={handleReset}
                    disabled={appState === STATES.VERIFYING}
                  >
                    Change Email
                  </button>
                </div>
              </form>
            )}
          </>
        )}
      </div>
    </div>
  )
}

export default App

(function () {
  'use strict'

  const script = document.currentScript
  const defaults = {
    apiBase: script && script.src ? new URL(script.src).origin : location.origin,
    callerUserId: '', token: '', tokenExpiresAt: 0, tokenEndpoint: '', appId: '', appSecret: '', channelId: 'h5', tenantId: '',
    title: '在线客服', greeting: '您好，我是智能客服，请问有什么可以帮您？',
    position: 'right', primaryColor: '#246bfd', zIndex: 2147483000,
  }

  function escapeHTML(value) {
    const node = document.createElement('div')
    node.textContent = String(value == null ? '' : value)
    return node.innerHTML
  }

  function icon(name) {
    const icons = {
      chat: '<svg viewBox="0 0 24 24"><path d="M21 11.5a8.4 8.4 0 0 1-9 8.5 9.5 9.5 0 0 1-4-.9L3 21l1.7-4.5A8.3 8.3 0 0 1 3 11.5C3 6.8 7 3 12 3s9 3.8 9 8.5Z"/></svg>',
      close: '<svg viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18"/></svg>',
      history: '<svg viewBox="0 0 24 24"><path d="M3 12a9 9 0 1 0 3-6.7L3 8M3 3v5h5M12 7v5l3 2"/></svg>',
      plus: '<svg viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></svg>',
      back: '<svg viewBox="0 0 24 24"><path d="m15 18-6-6 6-6"/></svg>',
      send: '<svg viewBox="0 0 24 24"><path d="m4 4 17 8-17 8 3-8-3-8Zm3 8h14"/></svg>',
    }
    return icons[name]
  }

  class CustomerServiceWidget extends HTMLElement {
    static get observedAttributes() { return ['caller-user-id', 'token', 'api-base', 'token-endpoint', 'app-id', 'app-secret'] }

    constructor() {
      super()
      this.attachShadow({ mode: 'open' })
      this.config = Object.assign({}, defaults)
      this.state = { open: false, view: 'chat', sessionId: '', messages: [], sessions: [], busy: false, error: '' }
    }

    connectedCallback() {
      this.readAttributes()
      this.render()
      this.bind()
      this.restoreSession()
    }

    attributeChangedCallback() { if (this.isConnected) this.readAttributes() }

    configure(options) {
      Object.assign(this.config, options || {})
      if (this.isConnected) { this.render(); this.bind(); this.restoreSession() }
      return this
    }

    open() { this.state.open = true; this.render(); this.bind(); this.ensureSession() }
    close() { this.state.open = false; this.render(); this.bind() }

    readAttributes() {
      const map = { 'api-base':'apiBase', 'caller-user-id':'callerUserId', token:'token', 'token-endpoint':'tokenEndpoint', 'app-id':'appId', 'app-secret':'appSecret', 'channel-id':'channelId', 'tenant-id':'tenantId', title:'title', greeting:'greeting', position:'position', 'primary-color':'primaryColor' }
      Object.entries(map).forEach(([attr, key]) => { if (this.hasAttribute(attr)) this.config[key] = this.getAttribute(attr) })
      this.config.apiBase = String(this.config.apiBase || location.origin).replace(/\/$/, '')
    }

    storageKey() { return `cs-widget:${this.config.tenantId || '_'}:${this.config.channelId || '_'}:${this.config.callerUserId || '_'}` }

    async restoreSession() {
      if (!this.config.callerUserId) return
      const saved = localStorage.getItem(this.storageKey())
      if (!saved) return
      this.state.sessionId = saved
      try { await this.loadHistory(saved) } catch { localStorage.removeItem(this.storageKey()); this.state.sessionId = '' }
    }

    headers() {
      const h = { 'Content-Type': 'application/json', Authorization: `Bearer ${this.config.token}` }
      if (this.config.callerUserId) h['X-Caller-User-Id'] = this.config.callerUserId
      if (this.config.channelId) h['X-Channel-Id'] = this.config.channelId
      if (this.config.tenantId) h['X-Tenant-Id'] = this.config.tenantId
      return h
    }

    async ensureToken() {
      if (this.config.token && (!this.config.tokenExpiresAt || Date.now() < this.config.tokenExpiresAt - 60000)) return this.config.token
      if (!this.config.callerUserId) throw new Error('缺少 caller-user-id')
      const endpoint = this.config.tokenEndpoint || `${this.config.apiBase}/api/v1/biz/auth/token`
      const payload = { caller_user_id: this.config.callerUserId, channel_id: this.config.channelId, tenant_id: this.config.tenantId }
      if (this.config.appId) payload.app_id = this.config.appId
      if (this.config.appSecret) payload.app_secret = this.config.appSecret
      const res = await fetch(endpoint, { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(payload), credentials: this.config.tokenEndpoint ? 'include' : 'omit' })
      if (!res.ok) { let msg = `获取客服凭证失败 (${res.status})`; try { msg = (await res.json()).message || msg } catch {} throw new Error(msg) }
      const body = await res.json(), data = body.data || body
      this.config.token = data.access_token || data.token
      if (!this.config.token) throw new Error('凭证接口未返回 access_token')
      this.config.tokenExpiresAt = data.expires_at ? new Date(data.expires_at).getTime() : Date.now() + Number(data.expires_in || 900) * 1000
      return this.config.token
    }

    async request(path, options) {
      await this.ensureToken()
      const res = await fetch(`${this.config.apiBase}/api/v1/biz${path}`, Object.assign({ headers: this.headers() }, options))
      if (!res.ok) { let msg = `请求失败 (${res.status})`; try { msg = (await res.json()).message || msg } catch {} throw new Error(msg) }
      const body = await res.json()
      return body.data
    }

    validate() {
      if (!this.config.callerUserId) throw new Error('缺少 caller-user-id')
      if (!this.config.token && !this.config.tokenEndpoint && !(this.config.appId && this.config.appSecret)) throw new Error('缺少 token-endpoint 或应用凭证')
    }

    async ensureSession() {
      if (this.state.sessionId || this.state.busy) return
      await this.newSession(false)
    }

    async newSession(confirmFirst) {
      if (confirmFirst && this.state.messages.length && !window.confirm('确定开始一个新对话吗？当前对话仍可在历史记录中查看。')) return
      try {
        this.validate(); this.setBusy(true)
        const data = await this.request('/sessions', { method: 'POST', body: JSON.stringify({ caller_user_id: this.config.callerUserId, channel_id: this.config.channelId, tenant_id: this.config.tenantId, meta: { source: 'h5-widget', page_url: location.href } }) })
        this.state.sessionId = data.id || data.session_id
        this.state.messages = []
        this.state.view = 'chat'
        localStorage.setItem(this.storageKey(), this.state.sessionId)
        this.render(); this.bind()
      } catch (e) { this.showError(e.message) } finally { this.setBusy(false) }
    }

    async loadHistory(id) {
      this.setBusy(true)
      try {
        const data = await this.request(`/sessions/${encodeURIComponent(id)}/history`)
        this.state.sessionId = id
        this.state.messages = (data && data.messages) || (Array.isArray(data) ? data : [])
        this.state.view = 'chat'
        localStorage.setItem(this.storageKey(), id)
        this.render(); this.bind(); this.scrollBottom()
      } finally { this.setBusy(false) }
    }

    async loadSessions() {
      try {
        this.validate(); this.state.view = 'history'; this.setBusy(true)
        const query = new URLSearchParams({ page: '1', size: '30' })
        if (this.config.channelId) query.set('channel_id', this.config.channelId)
        if (this.config.tenantId) query.set('tenant_id', this.config.tenantId)
        const data = await this.request(`/users/${encodeURIComponent(this.config.callerUserId)}/sessions?${query}`)
        this.state.sessions = (data && (data.items || data.list || data.sessions)) || (Array.isArray(data) ? data : [])
        this.render(); this.bind()
      } catch (e) { this.showError(e.message) } finally { this.setBusy(false) }
    }

    async send() {
      const input = this.shadowRoot.querySelector('.input')
      const content = input.value.trim()
      if (!content || this.state.busy) return
      try {
        this.validate(); await this.ensureToken(); if (!this.state.sessionId) await this.newSession(false)
        input.value = ''
        this.state.messages.push({ role: 'user', content }, { role: 'assistant', content: '', streaming: true })
        this.setBusy(true); this.render(); this.bind(); this.scrollBottom()
        const assistant = this.state.messages[this.state.messages.length - 1]
        const res = await fetch(`${this.config.apiBase}/api/v1/biz/sessions/${encodeURIComponent(this.state.sessionId)}/chat`, {
          method: 'POST', headers: this.headers(), body: JSON.stringify({ message: content, client_message_id: crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}` }),
        })
        if (!res.ok) { let msg = `请求失败 (${res.status})`; try { msg = (await res.json()).message || msg } catch {} throw new Error(msg) }
        await this.readStream(res, ev => {
          if (ev.content && ['stream_chunk','message','text'].includes(ev.type)) assistant.content += ev.content
          if (ev.error) assistant.content = `抱歉，服务暂时不可用：${ev.error}`
          this.paintMessages(); this.scrollBottom()
        })
        assistant.streaming = false
        if (!assistant.content) assistant.content = '抱歉，我暂时没有生成有效回复，请稍后再试。'
      } catch (e) {
        const last = this.state.messages[this.state.messages.length - 1]
        if (last && last.role === 'assistant' && !last.content) last.content = `发送失败：${e.message}`
        else this.showError(e.message)
      } finally { this.setBusy(false); this.paintMessages(); this.bind(); this.scrollBottom() }
    }

    async readStream(res, onEvent) {
      if (!res.body) throw new Error('当前浏览器不支持流式回复')
      const reader = res.body.getReader(), decoder = new TextDecoder(), flush = text => {
        text.split('\n').forEach(line => { if (!line.trim().startsWith('data:')) return; const raw = line.trim().slice(5).trim(); if (!raw || raw === '[DONE]') return; try { onEvent(JSON.parse(raw)) } catch {} })
      }
      let buffer = ''
      while (true) { const { done, value } = await reader.read(); if (done) break; buffer += decoder.decode(value, {stream:true}); const blocks = buffer.split(/\r?\n\r?\n/); buffer = blocks.pop() || ''; blocks.forEach(flush) }
      if (buffer) flush(buffer)
    }

    setBusy(value) { this.state.busy = value; const send = this.shadowRoot.querySelector('.send'); if (send) send.disabled = value }
    showError(message) { this.state.error = message; this.render(); this.bind(); setTimeout(() => { this.state.error = ''; if (this.isConnected) { this.render(); this.bind() } }, 4000) }
    scrollBottom() { requestAnimationFrame(() => { const el = this.shadowRoot.querySelector('.messages'); if (el) el.scrollTop = el.scrollHeight }) }
    formatTime(value) { if (!value) return ''; const d = new Date(value); return isNaN(d) ? '' : d.toLocaleString([], {month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit'}) }

    paintMessages() {
      const wrap = this.shadowRoot.querySelector('.messages')
      if (!wrap) return
      wrap.innerHTML = this.messagesHTML()
    }

    messagesHTML() {
      const greeting = this.state.messages.length ? '' : `<div class="message assistant"><div class="avatar">客</div><div><div class="bubble">${escapeHTML(this.config.greeting)}</div></div></div>`
      return greeting + this.state.messages.filter(m => m.role !== 'system').map(m => `<div class="message ${m.role === 'user' ? 'user' : 'assistant'}">${m.role === 'user' ? '' : '<div class="avatar">客</div>'}<div><div class="bubble">${escapeHTML(m.content || '').replace(/\n/g,'<br>')}${m.streaming && !m.content ? '<span class="typing"><i></i><i></i><i></i></span>' : ''}</div><time>${this.formatTime(m.created_at)}</time></div></div>`).join('')
    }

    historyHTML() {
      if (!this.state.sessions.length && !this.state.busy) return '<div class="empty"><div>暂无历史对话</div><small>开始咨询后，会话会显示在这里</small></div>'
      return `<div class="session-list">${this.state.sessions.map(s => `<button class="session" data-id="${escapeHTML(s.id || s.session_id)}"><span class="session-title">${escapeHTML(s.title || s.last_message_preview || '新对话')}</span><span class="session-preview">${escapeHTML(s.last_message_preview || '暂无消息')}</span><time>${this.formatTime(s.last_message_at || s.updated_at || s.created_at)}</time></button>`).join('')}</div>`
    }

    render() {
      const side = this.config.position === 'left' ? 'left:20px' : 'right:20px'
      this.shadowRoot.innerHTML = `<style>${this.styles()}</style><div class="root" style="--primary:${escapeHTML(this.config.primaryColor)};--z:${Number(this.config.zIndex)||2147483000};${side}">
        ${this.state.open ? `<section class="panel" role="dialog" aria-label="${escapeHTML(this.config.title)}">
          <header><div class="agent-mark">客<span></span></div><div class="heading"><strong>${escapeHTML(this.config.title)}</strong><small>在线 · 随时为您服务</small></div><button class="icon-btn close" aria-label="关闭">${icon('close')}</button></header>
          <nav><button class="back ${this.state.view === 'history' ? '' : 'hidden'}" aria-label="返回">${icon('back')}</button><span>${this.state.view === 'history' ? '历史对话' : ''}</span><div><button class="icon-btn history" aria-label="历史对话">${icon('history')}</button><button class="icon-btn new" aria-label="新对话">${icon('plus')}</button></div></nav>
          ${this.state.error ? `<div class="error">${escapeHTML(this.state.error)}</div>` : ''}
          ${this.state.view === 'history' ? `<main class="history-view">${this.historyHTML()}</main>` : `<main class="messages" aria-live="polite">${this.messagesHTML()}</main><footer><textarea class="input" rows="1" maxlength="2000" placeholder="输入您想咨询的问题…" aria-label="消息内容"></textarea><button class="send" aria-label="发送" ${this.state.busy?'disabled':''}>${icon('send')}</button><small>AI 回答仅供参考</small></footer>`}
        </section>` : ''}
        <button class="launcher" aria-label="${this.state.open ? '关闭客服' : '打开在线客服'}">${icon(this.state.open ? 'close' : 'chat')}<span>${this.state.open ? '' : '在线客服'}</span></button>
      </div>`
    }

    bind() {
      const $ = s => this.shadowRoot.querySelector(s)
      $('.launcher')?.addEventListener('click', () => this.state.open ? this.close() : this.open())
      $('.close')?.addEventListener('click', () => this.close())
      $('.history')?.addEventListener('click', () => this.loadSessions())
      $('.new')?.addEventListener('click', () => this.newSession(true))
      $('.back')?.addEventListener('click', () => { this.state.view='chat'; this.render(); this.bind(); this.scrollBottom() })
      $('.send')?.addEventListener('click', () => this.send())
      $('.input')?.addEventListener('keydown', e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); this.send() } })
      this.shadowRoot.querySelectorAll('.session').forEach(el => el.addEventListener('click', () => this.loadHistory(el.dataset.id).catch(e => this.showError(e.message))))
    }

    styles() { return `
      *{box-sizing:border-box}button,textarea{font:inherit}.root{position:fixed;bottom:20px;z-index:var(--z);font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;color:#172033}.launcher{height:54px;min-width:54px;padding:0 18px;border:0;border-radius:28px;background:var(--primary);color:#fff;box-shadow:0 10px 28px color-mix(in srgb,var(--primary) 35%,transparent);display:flex;align-items:center;justify-content:center;gap:8px;cursor:pointer;font-weight:600}.launcher:hover{filter:brightness(1.06);transform:translateY(-1px)}svg{width:22px;height:22px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}.panel{position:absolute;bottom:68px;right:0;width:min(380px,calc(100vw - 24px));height:min(620px,calc(100vh - 112px));background:#f5f7fb;border:1px solid rgba(22,34,58,.08);border-radius:20px;box-shadow:0 24px 70px rgba(27,43,75,.22);overflow:hidden;display:flex;flex-direction:column;animation:pop .18s ease-out}@keyframes pop{from{opacity:0;transform:translateY(10px) scale(.98)}}header{height:76px;padding:0 16px 0 18px;background:linear-gradient(135deg,var(--primary),color-mix(in srgb,var(--primary) 74%,#7a5cff));color:white;display:flex;align-items:center;gap:12px}.agent-mark{width:40px;height:40px;border-radius:14px;background:rgba(255,255,255,.2);display:grid;place-items:center;font-weight:700;position:relative}.agent-mark span{position:absolute;width:9px;height:9px;border:2px solid white;background:#35d07f;border-radius:50%;right:-1px;bottom:-1px}.heading{display:flex;flex:1;flex-direction:column;gap:3px}.heading strong{font-size:16px}.heading small{font-size:12px;opacity:.84}.icon-btn,.back{border:0;background:transparent;color:inherit;width:36px;height:36px;border-radius:11px;display:grid;place-items:center;cursor:pointer}.icon-btn:hover,.back:hover{background:rgba(127,140,166,.12)}header .icon-btn:hover{background:rgba(255,255,255,.15)}nav{height:48px;padding:0 10px;display:flex;align-items:center;justify-content:space-between;background:#fff;border-bottom:1px solid #e9edf4;color:#667085;font-size:14px}nav>div{display:flex}.hidden{visibility:hidden}.messages{flex:1;overflow:auto;padding:20px 16px;scroll-behavior:smooth}.message{display:flex;gap:9px;margin-bottom:18px;align-items:flex-start}.message.user{justify-content:flex-end}.avatar{flex:0 0 30px;height:30px;border-radius:10px;background:linear-gradient(135deg,var(--primary),#7759ec);color:#fff;display:grid;place-items:center;font-size:12px}.bubble{max-width:270px;padding:11px 13px;border-radius:4px 15px 15px;background:#fff;box-shadow:0 3px 12px rgba(32,47,77,.06);font-size:14px;line-height:1.6;word-break:break-word}.user .bubble{background:var(--primary);color:#fff;border-radius:15px 4px 15px 15px}.message time{display:block;color:#a2aabb;font-size:10px;margin-top:4px}.user time{text-align:right}.typing{display:inline-flex;gap:4px;padding:5px}.typing i{width:5px;height:5px;border-radius:50%;background:#9ba6b8;animation:b 1s infinite}.typing i:nth-child(2){animation-delay:.15s}.typing i:nth-child(3){animation-delay:.3s}@keyframes b{50%{transform:translateY(-3px)}}footer{background:white;padding:12px 12px 8px;border-top:1px solid #e9edf4;display:grid;grid-template-columns:1fr 42px;gap:8px}.input{resize:none;border:0;outline:0;background:#f4f6fa;border-radius:12px;padding:11px 12px;line-height:20px;max-height:86px;color:#222}.send{border:0;border-radius:12px;background:var(--primary);color:white;display:grid;place-items:center;cursor:pointer}.send:disabled{opacity:.45;cursor:not-allowed}footer small{grid-column:1/-1;text-align:center;color:#a0a8b7;font-size:10px}.history-view{flex:1;overflow:auto;background:#fff}.session-list{padding:8px 14px}.session{width:100%;border:0;border-bottom:1px solid #edf0f5;background:#fff;text-align:left;padding:14px 4px;display:grid;grid-template-columns:1fr auto;gap:5px 12px;cursor:pointer;color:#172033}.session:hover{background:#f8faff}.session-title{font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.session-preview{font-size:12px;color:#838da0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.session time{grid-column:2;grid-row:1;color:#9aa3b3;font-size:10px}.empty{height:100%;display:grid;place-content:center;text-align:center;gap:7px;color:#7e899c}.empty small{color:#abb3c0}.error{position:absolute;top:82px;left:20px;right:20px;padding:10px 12px;background:#fff1f1;color:#b42318;border-radius:10px;box-shadow:0 8px 20px rgba(50,30,30,.12);z-index:2;font-size:13px}
      @media(max-width:480px){.root{right:12px!important;left:12px!important;bottom:12px}.panel{position:fixed;inset:0;width:100%;height:100%;max-height:none;border-radius:0}.launcher{margin-left:auto}.bubble{max-width:calc(100vw - 100px)}}
      @media(prefers-reduced-motion:reduce){.panel{animation:none}.launcher{transition:none}}
    ` }
  }

  if (!customElements.get('customer-service-widget')) customElements.define('customer-service-widget', CustomerServiceWidget)
  window.CustomerServiceWidget = { create(options) { const el = document.createElement('customer-service-widget'); document.body.appendChild(el); return el.configure(options) } }
})()

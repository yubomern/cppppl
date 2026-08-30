// Shared auth helpers, included on every page.
// - On login.html / register.html: does nothing extra (forms handle themselves).
// - On index.html: guards the page (redirects to /login.html if not authenticated)
//   and wires up the logout button + "logged in as ..." label.

const AUTH = {
    async checkSession() {
        const res = await fetch('/api/session', { credentials: 'include' });
        return res.json();
    },

    async logout() {
        await fetch('/api/logout', { method: 'POST', credentials: 'include' });
        window.location.href = '/login.html';
    },

    async guardPage() {
        const session = await this.checkSession();
        if (!session.authenticated) {
            window.location.href = '/login.html';
            return null;
        }
        const whoami = document.getElementById('whoami');
        if (whoami) whoami.textContent = 'Connecté en tant que ' + session.username;
        const logoutBtn = document.getElementById('logoutBtn');
        if (logoutBtn) logoutBtn.addEventListener('click', () => this.logout());
        return session;
    }
};

// Only guard on pages that have the dashboard elements (index.html).
if (document.getElementById('whoami')) {
    AUTH.guardPage();
}

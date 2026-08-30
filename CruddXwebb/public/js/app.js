// CRUD dashboard logic for index.html.
// Talks to the REST API: GET/POST /api/items, PUT/DELETE /api/items/{id}

const API = '/api/items';

async function fetchItems() {
    const res = await fetch(API, { credentials: 'include' });
    return res.json();
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str ?? '';
    return div.innerHTML;
}

function renderRow(item) {
    const tr = document.createElement('tr');
    tr.dataset.id = item.id;
    tr.innerHTML = `
        <td>${item.id}</td>
        <td class="cell-title">${escapeHtml(item.title)}</td>
        <td class="cell-description">${escapeHtml(item.description || '')}</td>
        <td>${escapeHtml(item.owner || '')}</td>
        <td class="actions">
            <button class="btn btn-primary btn-sm edit-btn">Modifier</button>
            <button class="btn btn-danger btn-sm delete-btn">Supprimer</button>
        </td>
    `;

    tr.querySelector('.delete-btn').addEventListener('click', () => deleteItem(item.id));
    tr.querySelector('.edit-btn').addEventListener('click', () => enterEditMode(tr, item));

    return tr;
}

function enterEditMode(tr, item) {
    const titleCell = tr.querySelector('.cell-title');
    const descCell = tr.querySelector('.cell-description');
    const actionsCell = tr.querySelector('.actions');

    titleCell.innerHTML = `<input class="edit-input" value="${escapeHtml(item.title)}">`;
    descCell.innerHTML = `<input class="edit-input" value="${escapeHtml(item.description || '')}">`;
    actionsCell.innerHTML = `
        <button class="btn btn-success btn-sm save-btn">Enregistrer</button>
        <button class="btn btn-ghost btn-sm cancel-btn" style="color:#333;background:#eee;">Annuler</button>
    `;

    actionsCell.querySelector('.save-btn').addEventListener('click', async () => {
        const newTitle = titleCell.querySelector('input').value.trim();
        const newDesc = descCell.querySelector('input').value.trim();
        if (!newTitle) return;
        await updateItem(item.id, { title: newTitle, description: newDesc });
        await loadItems();
    });

    actionsCell.querySelector('.cancel-btn').addEventListener('click', loadItems);
}

async function loadItems() {
    const items = await fetchItems();
    const tbody = document.getElementById('itemsBody');
    const emptyState = document.getElementById('emptyState');
    tbody.innerHTML = '';

    if (!items.length) {
        emptyState.style.display = 'block';
        return;
    }
    emptyState.style.display = 'none';
    items.forEach(item => tbody.appendChild(renderRow(item)));
}

async function createItem(title, description) {
    await fetch(API, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ title, description })
    });
}

async function updateItem(id, patch) {
    await fetch(`${API}/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(patch)
    });
}

async function deleteItem(id) {
    if (!confirm('Supprimer cet élément ?')) return;
    await fetch(`${API}/${id}`, { method: 'DELETE', credentials: 'include' });
    await loadItems();
}

document.getElementById('createForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const title = document.getElementById('newTitle').value.trim();
    const description = document.getElementById('newDescription').value.trim();
    if (!title) return;
    await createItem(title, description);
    document.getElementById('newTitle').value = '';
    document.getElementById('newDescription').value = '';
    await loadItems();
});

loadItems();

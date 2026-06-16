<script setup lang="ts">
import { ref } from 'vue'

const users = ref([
  { id: 1, name: 'Demo User', email: 'demo@lesstruct.com', role: 'Admin' },
])
</script>

<template>
  <div class="users-view">
    <h1 class="users-view__title">Users</h1>
    <div class="users-view__content">
      <div class="users-view__card">
        <h2 class="users-view__card-title">User Management</h2>
        <p class="users-view__description">Manage user accounts and permissions.</p>
        <table class="users-view__table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Role</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td>{{ user.name }}</td>
              <td>{{ user.email }}</td>
              <td>{{ user.role }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.users-view {
  padding: 1.5rem;
}

.users-view__title {
  font-size: 1.875rem;
  font-weight: 700;
  color: var(--brand-dark-2);
  margin: 0 0 1.5rem 0;
}

.users-view__content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.users-view__card {
  background-color: var(--color-white);
  border: 1px solid var(--brand-light-2);
  border-radius: 0.5rem;
  padding: 1.5rem;
}

.users-view__card-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--brand-dark-2);
  margin: 0 0 0.5rem 0;
}

.users-view__description {
  color: var(--color-text-muted);
  margin: 0 0 1rem 0;
}

.users-view__table {
  width: 100%;
  border-collapse: collapse;
}

.users-view__table th,
.users-view__table td {
  padding: 0.75rem;
  text-align: left;
  border-bottom: 1px solid var(--brand-light-2);
}

.users-view__table th {
  font-weight: 600;
  color: var(--color-text-secondary);
  background-color: var(--brand-light-1);
}

.users-view__table td {
  color: var(--color-text-muted);
}

.users-view__table tr:last-child td {
  border-bottom: none;
}
</style>

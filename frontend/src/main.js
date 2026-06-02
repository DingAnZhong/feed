import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const routes = [
  { path: '/', redirect: '/feed' },
  { path: '/login', name: 'Login', component: () => import('./views/Login.vue') },
  { path: '/feed', name: 'Feed', component: () => import('./views/Feed.vue'), meta: { auth: true } },
  { path: '/publish', name: 'Publish', component: () => import('./views/Publish.vue'), meta: { auth: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.meta.auth && !localStorage.getItem('feed_user_id')) {
    return '/login'
  }
})

createApp(App).use(router).mount('#app')

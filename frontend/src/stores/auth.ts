import { createSignal } from 'solid-js';
import { authAPI, type User } from '../services/api';

const [user, setUser] = createSignal<User | null>(null);
const [isAuthenticated, setIsAuthenticated] = createSignal(false);
const [loading, setLoading] = createSignal(true);

export const authStore = {
  user,
  isAuthenticated,
  loading,
  
  async checkAuth() {
    const token = localStorage.getItem('token');
    if (!token) {
      setLoading(false);
      return;
    }

    try {
      const response = await authAPI.me();
      setUser(response.data);
      setIsAuthenticated(true);
    } catch (error) {
      localStorage.removeItem('token');
      setIsAuthenticated(false);
    } finally {
      setLoading(false);
    }
  },

  login() {
    authAPI.login().then((response) => {
      window.location.href = response.data.url;
    });
  },

  logout() {
    localStorage.removeItem('token');
    setUser(null);
    setIsAuthenticated(false);
    window.location.href = '/';
  },

  async setToken(token: string) {
    localStorage.setItem('token', token);
    await this.checkAuth();
  },
};
import { type Component, onMount } from 'solid-js';
import { useSearchParams, useNavigate } from '@solidjs/router';
import { authStore } from '../stores/auth';

const Login: Component = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  onMount(async () => {
    const token = searchParams.token as string | undefined;
    if (token) {
      await authStore.setToken(token);

      const redirectPath = localStorage.getItem('redirect_after_login');
      localStorage.removeItem('redirect_after_login');

      navigate(redirectPath || '/dashboard');
    }
  });

  return (
    <div class="container mx-auto px-4 py-16 text-center">
      <h2 class="text-2xl mb-4">Logging in...</h2>
    </div>
  );
};

export default Login;
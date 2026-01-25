import { type Component, Show } from 'solid-js';
import { A } from '@solidjs/router';
import { authStore } from '../stores/auth';

const Navbar: Component = () => {
  return (
    <nav class="bg-gray-800 text-white p-4">
      <div class="container mx-auto flex justify-between items-center">
        <A href="/" class="text-xl font-bold">
          Gitea Classroom
        </A>
        
        <div class="flex gap-4 items-center">
          <Show
            when={authStore.isAuthenticated()}
            fallback={
              <button
                onClick={() => authStore.login()}
                class="bg-blue-600 hover:bg-blue-700 px-4 py-2 rounded"
              >
                Login with Gitea
              </button>
            }
          >
            <A href="/dashboard" class="hover:text-gray-300">
              Dashboard
            </A>
            <div class="flex items-center gap-2">
              <Show when={authStore.user()}>
                {(user) => (
                  <>
                    <img
                      src={user().avatar_url}
                      alt={user().username}
                      class="w-8 h-8 rounded-full"
                    />
                    <span>{user().username}</span>
                  </>
                )}
              </Show>
              <button
                onClick={() => authStore.logout()}
                class="bg-red-600 hover:bg-red-700 px-4 py-2 rounded ml-2"
              >
                Logout
              </button>
            </div>
          </Show>
        </div>
      </div>
    </nav>
  );
};

export default Navbar;
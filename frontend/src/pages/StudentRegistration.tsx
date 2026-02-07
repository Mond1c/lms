import {
  type Component,
  createResource,
  Show,
  For,
  createSignal,
} from 'solid-js';
import { useParams } from '@solidjs/router';
import { inviteAPI, type StudentInvite } from '../services/api';

const StudentRegistrationPage: Component = () => {
  const params = useParams();

  const [data] = createResource(() => params.code, async (code) => {
    const response = await inviteAPI.getAvailableStudents(code);
    return response.data;
  });

  const [selectedInvite, setSelectedInvite] = createSignal<number | null>(null);
  const [email, setEmail] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [confirmPassword, setConfirmPassword] = createSignal('');
  const [registering, setRegistering] = createSignal(false);
  const [error, setError] = createSignal('');
  const [success, setSuccess] = createSignal(false);
  const [username, setUsername] = createSignal('');

  const handleRegister = async (e: Event) => {
    e.preventDefault();

    if (!selectedInvite()) {
      setError('Выберите ваше ФИО из списка');
      return;
    }

    if (!email() || !password() || !confirmPassword()) {
      setError('Заполните все поля');
      return;
    }

    if (password() !== confirmPassword()) {
      setError('Пароли не совпадают');
      return;
    }

    if (password().length < 8) {
      setError('Пароль должен быть не менее 8 символов');
      return;
    }

    setRegistering(true);
    setError('');

    try {
      const response = await inviteAPI.register(params.code, {
        invite_id: selectedInvite()!,
        email: email(),
        password: password(),
      });

      setUsername(response.data.username);
      setSuccess(true);

      // Redirect after 3 seconds
      setTimeout(() => {
        window.location.href = response.data.redirect;
      }, 3000);
    } catch (err: any) {
      setError(
        err.response?.data?.message ||
          'Ошибка при регистрации. Попробуйте еще раз.'
      );
    } finally {
      setRegistering(false);
    }
  };

  return (
    <div class="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 py-12 px-4">
      <div class="max-w-2xl mx-auto">
        <Show
          when={!data.loading && data()}
          fallback={
            <div class="bg-white rounded-lg shadow-lg p-8 text-center">
              <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
              <p class="mt-4 text-gray-600">Загрузка...</p>
            </div>
          }
        >
          {(d) => (
            <>
              <Show
                when={!success()}
                fallback={
                  <div class="bg-white rounded-lg shadow-lg p-8">
                    <div class="text-center">
                      <div class="mx-auto flex items-center justify-center h-16 w-16 rounded-full bg-green-100 mb-4">
                        <svg
                          class="h-10 w-10 text-green-600"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      </div>
                      <h2 class="text-2xl font-bold text-gray-900 mb-2">
                        Регистрация завершена!
                      </h2>
                      <p class="text-gray-600 mb-4">
                        Ваш аккаунт успешно создан
                      </p>
                      <div class="bg-blue-50 rounded-lg p-4 mb-4">
                        <p class="text-sm text-gray-700 mb-1">
                          Ваш username:
                        </p>
                        <p class="text-lg font-mono font-semibold text-blue-600">
                          {username()}
                        </p>
                      </div>
                      <p class="text-sm text-gray-500">
                        Перенаправление на страницу входа...
                      </p>
                    </div>
                  </div>
                }
              >
                <div class="bg-white rounded-lg shadow-lg p-8">
                  <div class="mb-6">
                    <h1 class="text-3xl font-bold text-gray-900 mb-2">
                      Регистрация в курсе
                    </h1>
                    <p class="text-lg text-gray-600">{d().course_name}</p>
                  </div>

                  <Show when={error()}>
                    <div class="mb-4 p-4 bg-red-50 border border-red-200 text-red-700 rounded">
                      {error()}
                    </div>
                  </Show>

                  <form onSubmit={handleRegister}>
                    {/* Select Name */}
                    <div class="mb-6">
                      <label class="block text-sm font-medium text-gray-700 mb-2">
                        Выберите ваше ФИО из списка
                      </label>
                      <div class="border rounded-lg divide-y max-h-64 overflow-y-auto">
                        <For each={d().students}>
                          {(student) => (
                            <label
                              class={`flex items-center p-3 cursor-pointer hover:bg-gray-50 ${
                                student.used ? 'opacity-50 cursor-not-allowed' : ''
                              }`}
                            >
                              <input
                                type="radio"
                                name="student"
                                value={student.id}
                                disabled={student.used}
                                checked={selectedInvite() === student.id}
                                onChange={() => setSelectedInvite(student.id)}
                                class="mr-3"
                              />
                              <span class="flex-1">{student.full_name}</span>
                              <Show when={student.used}>
                                <span class="text-xs text-gray-500">
                                  (уже зарегистрирован)
                                </span>
                              </Show>
                            </label>
                          )}
                        </For>
                      </div>
                    </div>

                    {/* Email */}
                    <div class="mb-4">
                      <label
                        for="email"
                        class="block text-sm font-medium text-gray-700 mb-2"
                      >
                        Email
                      </label>
                      <input
                        id="email"
                        type="email"
                        required
                        value={email()}
                        onInput={(e) => setEmail(e.currentTarget.value)}
                        class="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="your.email@example.com"
                      />
                    </div>

                    {/* Password */}
                    <div class="mb-4">
                      <label
                        for="password"
                        class="block text-sm font-medium text-gray-700 mb-2"
                      >
                        Пароль
                      </label>
                      <input
                        id="password"
                        type="password"
                        required
                        value={password()}
                        onInput={(e) => setPassword(e.currentTarget.value)}
                        class="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="Минимум 8 символов"
                      />
                    </div>

                    {/* Confirm Password */}
                    <div class="mb-6">
                      <label
                        for="confirm-password"
                        class="block text-sm font-medium text-gray-700 mb-2"
                      >
                        Подтвердите пароль
                      </label>
                      <input
                        id="confirm-password"
                        type="password"
                        required
                        value={confirmPassword()}
                        onInput={(e) =>
                          setConfirmPassword(e.currentTarget.value)
                        }
                        class="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                        placeholder="Повторите пароль"
                      />
                    </div>

                    <button
                      type="submit"
                      disabled={registering() || !selectedInvite()}
                      class="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 text-white font-medium py-3 px-4 rounded-lg transition-colors"
                    >
                      {registering()
                        ? 'Создание аккаунта...'
                        : 'Зарегистрироваться'}
                    </button>
                  </form>
                </div>
              </Show>
            </>
          )}
        </Show>
      </div>
    </div>
  );
};

export default StudentRegistrationPage;

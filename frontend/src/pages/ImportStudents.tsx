import { type Component, createResource, Show, For, createSignal } from 'solid-js';
import { useParams, A, useNavigate } from '@solidjs/router';
import { courseAPI, inviteAPI, type StudentInvite } from '../services/api';

const ImportStudentsPage: Component = () => {
  const params = useParams();
  const navigate = useNavigate();

  const [course] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await courseAPI.get(slug);
      return response.data;
    }
  );

  const [invites, { refetch }] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await inviteAPI.listInvites(slug);
      return response.data;
    }
  );

  const [textInput, setTextInput] = createSignal('');
  const [importing, setImporting] = createSignal(false);
  const [error, setError] = createSignal('');
  const [success, setSuccess] = createSignal('');

  const handleTextImport = async () => {
    const text = textInput().trim();
    if (!text) {
      setError('Введите список студентов');
      return;
    }

    const lines = text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    if (lines.length === 0) {
      setError('Список пуст');
      return;
    }

    setImporting(true);
    setError('');
    setSuccess('');

    try {
      await inviteAPI.importStudents(params.slug, lines);
      setSuccess(`Добавлено ${lines.length} студентов`);
      setTextInput('');
      refetch();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Ошибка при импорте');
    } finally {
      setImporting(false);
    }
  };

  const handleFileImport = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    setImporting(true);
    setError('');
    setSuccess('');

    try {
      await inviteAPI.importStudentsCSV(params.slug, file);
      setSuccess('Студенты успешно импортированы из CSV');
      input.value = '';
      refetch();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Ошибка при импорте CSV');
    } finally {
      setImporting(false);
    }
  };

  return (
    <div class="container mx-auto px-4 py-8">
      <Show
        when={!course.loading && course()}
        fallback={<div class="text-center py-8">Загрузка...</div>}
      >
        {(c) => (
          <>
            <div class="mb-8">
              <A
                href={`/courses/${c().slug}/students`}
                class="text-blue-600 hover:underline mb-2 inline-block"
              >
                ← Назад к списку студентов
              </A>
              <h1 class="text-3xl font-bold mb-2">Импорт студентов</h1>
              <p class="text-gray-600">
                Добавьте список студентов для курса "{c().name}"
              </p>
            </div>

            <Show when={error()}>
              <div class="mb-4 p-4 bg-red-50 border border-red-200 text-red-700 rounded">
                {error()}
              </div>
            </Show>

            <Show when={success()}>
              <div class="mb-4 p-4 bg-green-50 border border-green-200 text-green-700 rounded">
                {success()}
              </div>
            </Show>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
              {/* Text Input */}
              <div class="bg-white rounded-lg shadow p-6">
                <h2 class="text-xl font-semibold mb-4">Ввести список</h2>
                <p class="text-sm text-gray-600 mb-4">
                  Введите ФИО студентов, по одному на строку
                </p>
                <textarea
                  class="w-full border rounded p-3 min-h-[300px] font-mono text-sm"
                  placeholder={`Иванов Иван Иванович\nПетров Петр\nСидоров Сидор Сидорович`}
                  value={textInput()}
                  onInput={(e) => setTextInput(e.currentTarget.value)}
                  disabled={importing()}
                />
                <button
                  onClick={handleTextImport}
                  disabled={importing() || !textInput().trim()}
                  class="mt-4 w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 text-white px-4 py-2 rounded"
                >
                  {importing() ? 'Импорт...' : 'Импортировать'}
                </button>
              </div>

              {/* CSV Upload */}
              <div class="bg-white rounded-lg shadow p-6">
                <h2 class="text-xl font-semibold mb-4">Загрузить CSV</h2>
                <p class="text-sm text-gray-600 mb-4">
                  Загрузите CSV файл со списком студентов
                </p>
                <div class="border-2 border-dashed rounded-lg p-8 text-center">
                  <svg
                    class="mx-auto h-12 w-12 text-gray-400 mb-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                    />
                  </svg>
                  <label class="cursor-pointer">
                    <span class="text-blue-600 hover:underline">
                      Выберите файл
                    </span>
                    <input
                      type="file"
                      accept=".csv"
                      class="hidden"
                      onChange={handleFileImport}
                      disabled={importing()}
                    />
                  </label>
                </div>
                <div class="mt-4 text-sm text-gray-500">
                  <p class="font-medium mb-2">Формат CSV:</p>
                  <pre class="bg-gray-50 p-3 rounded text-xs">
                    {`ФИО\nИванов Иван Иванович\nПетров Петр\nСидоров Сидор Сидорович`}
                  </pre>
                </div>
              </div>
            </div>

            {/* List of invites */}
            <div class="bg-white rounded-lg shadow">
              <div class="px-6 py-4 border-b">
                <h2 class="text-xl font-semibold">
                  Добавленные студенты ({invites()?.length || 0})
                </h2>
              </div>
              <Show
                when={!invites.loading}
                fallback={<div class="p-6">Загрузка...</div>}
              >
                <Show
                  when={invites()?.length}
                  fallback={
                    <div class="p-6 text-center text-gray-500">
                      Студенты ещё не добавлены
                    </div>
                  }
                >
                  <table class="w-full">
                    <thead class="bg-gray-50">
                      <tr>
                        <th class="px-6 py-3 text-left text-sm font-medium text-gray-500">
                          ФИО
                        </th>
                        <th class="px-6 py-3 text-left text-sm font-medium text-gray-500">
                          Статус
                        </th>
                        <th class="px-6 py-3 text-left text-sm font-medium text-gray-500">
                          Username (если зарегистрирован)
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200">
                      <For each={invites()}>
                        {(invite) => (
                          <tr>
                            <td class="px-6 py-4">{invite.full_name}</td>
                            <td class="px-6 py-4">
                              <Show
                                when={invite.used}
                                fallback={
                                  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800">
                                    Ожидает регистрации
                                  </span>
                                }
                              >
                                <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800">
                                  Зарегистрирован
                                </span>
                              </Show>
                            </td>
                            <td class="px-6 py-4 text-gray-600">
                              {invite.student?.username || '-'}
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </Show>
              </Show>
            </div>
          </>
        )}
      </Show>
    </div>
  );
};

export default ImportStudentsPage;

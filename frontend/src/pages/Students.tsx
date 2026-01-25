import { type Component, createResource, Show, For } from 'solid-js';
import { useParams, A } from '@solidjs/router';
import { courseAPI, studentAPI } from '../services/api';

const StudentsPage: Component = () => {
  const params = useParams();

  const [course] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await courseAPI.get(slug);
      return response.data;
    }
  );

  const [students, { refetch }] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await studentAPI.list(slug);
      return response.data;
    }
  );

  const handleRemove = async (id: number) => {
    if (confirm('Are you sure you want to remove this student?')) {
      await studentAPI.remove(id);
      refetch();
    }
  };

  return (
    <div class="container mx-auto px-4 py-8">
      <Show
        when={!course.loading && course()}
        fallback={<div class="text-center py-8">Loading...</div>}
      >
        {(c) => (
          <>
            <div class="mb-8">
              <A
                href={`/courses/${c().slug}`}
                class="text-blue-600 hover:underline mb-2 inline-block"
              >
                Back to Course
              </A>
              <h1 class="text-3xl font-bold mb-2">Students - {c().name}</h1>
              <p class="text-gray-600">
                Manage enrolled students for this course.
              </p>
            </div>

            <div class="mb-6 p-4 bg-blue-50 rounded-lg">
              <h3 class="font-medium mb-2">Invite Link</h3>
              <div class="flex items-center gap-2">
                <code class="bg-white px-3 py-1 rounded border flex-1">
                  {window.location.origin}/join/{c().invite_code}
                </code>
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(
                      `${window.location.origin}/join/${c().invite_code}`
                    );
                  }}
                  class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-1 rounded"
                >
                  Copy
                </button>
              </div>
            </div>

            <Show
              when={!students.loading}
              fallback={<div>Loading students...</div>}
            >
              <Show
                when={students()?.length}
                fallback={
                  <div class="text-center py-8 text-gray-500">
                    No students enrolled yet. Share the invite link to add students.
                  </div>
                }
              >
                <div class="bg-white rounded-lg shadow overflow-hidden">
                  <table class="w-full">
                    <thead class="bg-gray-50">
                      <tr>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Student
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Username
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Email
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Actions
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200">
                      <For each={students()}>
                        {(student) => (
                          <tr>
                            <td class="px-4 py-3 font-medium">
                              {student.full_name || student.username}
                            </td>
                            <td class="px-4 py-3 text-gray-600">
                              {student.username}
                            </td>
                            <td class="px-4 py-3 text-gray-600">
                              {student.email}
                            </td>
                            <td class="px-4 py-3">
                              <button
                                onClick={() => handleRemove(student.id)}
                                class="text-red-600 hover:underline"
                              >
                                Remove
                              </button>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
                <div class="mt-4 text-gray-500 text-sm">
                  Total: {students()?.length} student(s)
                </div>
              </Show>
            </Show>
          </>
        )}
      </Show>
    </div>
  );
};

export default StudentsPage;

import { type Component, createResource, Show, For } from 'solid-js';
import { A } from '@solidjs/router';
import { courseAPI } from '../services/api';
import CourseCard from '../components/CourseCard';
import { authStore } from '../stores/auth';

const Dashboard: Component = () => {
  const [instructorCourses] = createResource(async () => {
    const response = await courseAPI.list();
    return response.data;
  });

  const [enrolledCourses] = createResource(async () => {
    const response = await courseAPI.listEnrolled();
    return response.data;
  });

  const isAdmin = () => authStore.user()?.is_admin || false;

  return (
    <div class="container mx-auto px-4 py-8">
      <div class="flex justify-between items-center mb-8">
        <h1 class="text-3xl font-bold">My Courses</h1>
        <Show when={isAdmin()}>
          <A
            href="/courses/create"
            class="bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded"
          >
            Create New Course
          </A>
        </Show>
      </div>

      <Show
        when={!instructorCourses.loading && !enrolledCourses.loading}
        fallback={<div class="text-center py-8">Loading courses...</div>}
      >
        <Show when={instructorCourses()?.length}>
          <div class="mb-8">
            <h2 class="text-xl font-semibold mb-4">Teaching</h2>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              <For each={instructorCourses()}>
                {(course) => <CourseCard course={course} />}
              </For>
            </div>
          </div>
        </Show>

        <Show when={enrolledCourses()?.length}>
          <div class="mb-8">
            <h2 class="text-xl font-semibold mb-4">Enrolled</h2>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              <For each={enrolledCourses()}>
                {(course) => <CourseCard course={course} />}
              </For>
            </div>
          </div>
        </Show>

        <Show when={!instructorCourses()?.length && !enrolledCourses()?.length}>
          <div class="text-center py-16 text-gray-500">
            <p class="text-xl mb-4">No courses yet</p>
            <p>Join a course using an invite link from your instructor.</p>
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default Dashboard;
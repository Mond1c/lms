import type { Component } from 'solid-js';
import { A } from '@solidjs/router';
import type { Course } from '../services/api';

interface CourseCardProps {
  course: Course;
}

const CourseCard: Component<CourseCardProps> = (props) => {
  return (
    <div class="bg-white rounded-lg shadow-md p-6 hover:shadow-lg transition-shadow">
      <h3 class="text-xl font-bold mb-2">{props.course.name}</h3>
      <p class="text-gray-600 mb-4">{props.course.description}</p>
      <div class="flex justify-between items-center">
        <span class="text-sm text-gray-500">
          Org: {props.course.org_name}
        </span>
        <A
          href={`/courses/${props.course.slug}`}
          class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded"
        >
          View Course
        </A>
      </div>
    </div>
  );
};

export default CourseCard;
/* @refresh reload */
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import './index.css';
import { authStore } from './stores/auth';
import { onMount } from 'solid-js';

import Navbar from './components/Navbar';
import Home from './pages/Home';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import CreateCourse from './pages/CreateCourse';
import Course from './pages/Course';
import Assignment from './pages/Assignment';
import JoinCourse from './pages/JoinCourse';
import Students from './pages/Students';
import AcceptAssignment from './pages/AcceptAssignment';

const root = document.getElementById('root');

if (!root) throw new Error('Root element not found');

const Layout = (props: { children?: any }) => {
  onMount(() => {
    authStore.checkAuth();
  });

  return (
    <div class="min-h-screen bg-gray-100">
      <Navbar />
      {props.children}
    </div>
  );
};

render(
  () => (
    <Router root={Layout}>
      <Route path="/" component={Home} />
      <Route path="/auth/callback" component={Login} />
      <Route path="/dashboard" component={Dashboard} />
      <Route path="/courses/create" component={CreateCourse} />
      <Route path="/courses/:slug" component={Course} />
      <Route path="/courses/:slug/students" component={Students} />
      <Route path="/assignments/:id" component={Assignment} />
      <Route path="/join/:code" component={JoinCourse} />
      <Route path="/accept/:id" component={AcceptAssignment} />
    </Router>
  ),
  root
);
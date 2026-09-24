import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import './app/styles/tokens.css';
import './app/styles/canvas-tokens.css';
import './app/styles/base.css';
import '@/shared/ui/styles/components.css';

import { App } from './app/App';

const root = document.getElementById('root');
if (!root) throw new Error('#root element is missing');

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

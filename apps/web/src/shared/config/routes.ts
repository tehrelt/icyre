/** Client routes. Screens not built yet render the "not built" page. */
export const routes = {
  home: '/',
  library: '/library',
  liked: '/liked',
  profile: '/profile',
  settings: '/settings',
  whatsNew: '/whats-new',
  history: '/history',
  /** "See all" views of a Home shelf. */
  shelf: (id: string) => `/browse/${encodeURIComponent(id)}`,
  search: '/search',
  genre: (slug: string) => `/genre/${encodeURIComponent(slug)}`,
  mood: (id: string) => `/mood/${encodeURIComponent(id)}`,
  album: (id: string) => `/album/${encodeURIComponent(id)}`,
  artist: (id: string) => `/artist/${encodeURIComponent(id)}`,
  playlist: (id: string) => `/playlist/${encodeURIComponent(id)}`,
} as const;

/* ICYRE design system — types as documentation. Global: window.Icyre (see bundle.js). */

// ---- icons ----
export declare const ICONS: {
    readonly play: "<path d=\"M7.5 5.8v12.4c0 .8.9 1.3 1.6.9l9.9-6.2c.6-.4.6-1.3 0-1.7L9.1 4.9c-.7-.4-1.6.1-1.6.9z\"/>";
    readonly pause: "<rect x=\"6.5\" y=\"5\" width=\"3.5\" height=\"14\" rx=\"1.2\"/><rect x=\"14\" y=\"5\" width=\"3.5\" height=\"14\" rx=\"1.2\"/>";
    readonly next: "<path d=\"M6 6.2v11.6c0 .7.8 1.1 1.4.7l8.1-5.8c.5-.4.5-1.1 0-1.4L7.4 5.5C6.8 5.1 6 5.5 6 6.2z\"/><path d=\"M18.5 5.5v13\"/>";
    readonly previous: "<path d=\"M18 6.2v11.6c0 .7-.8 1.1-1.4.7l-8.1-5.8c-.5-.4-.5-1.1 0-1.4l8.1-5.8c.6-.4 1.4 0 1.4.7z\"/><path d=\"M5.5 5.5v13\"/>";
    readonly shuffle: "<path d=\"M3.5 7h3.2c1.6 0 3 .8 3.9 2.1l2.8 5.8c.9 1.3 2.3 2.1 3.9 2.1h3.2\"/><path d=\"M3.5 17h3.2c1.2 0 2.3-.5 3.1-1.3\"/><path d=\"M13.7 8.3c.8-.8 1.9-1.3 3.1-1.3h3.7\"/><path d=\"M18 4.5 20.5 7 18 9.5\"/><path d=\"m18 14.5 2.5 2.5-2.5 2.5\"/>";
    readonly repeat: "<path d=\"M4 11V9.5A3.5 3.5 0 0 1 7.5 6H19\"/><path d=\"M16.5 3.5 19 6l-2.5 2.5\"/><path d=\"M20 13v1.5a3.5 3.5 0 0 1-3.5 3.5H5\"/><path d=\"M7.5 20.5 5 18l2.5-2.5\"/>";
    readonly volume: "<path d=\"M4 9.5v5c0 .6.4 1 1 1h2.5l4.3 3.6c.6.5 1.7.1 1.7-.8V5.7c0-.9-1-1.3-1.7-.8L7.5 8.5H5c-.6 0-1 .4-1 1z\"/><path d=\"M16.5 9a4.2 4.2 0 0 1 0 6\"/><path d=\"M19 6.5a7.8 7.8 0 0 1 0 11\"/>";
    readonly mute: "<path d=\"M4 9.5v5c0 .6.4 1 1 1h2.5l4.3 3.6c.6.5 1.7.1 1.7-.8V5.7c0-.9-1-1.3-1.7-.8L7.5 8.5H5c-.6 0-1 .4-1 1z\"/><path d=\"m16.5 9.5 5 5m0-5-5 5\"/>";
    readonly search: "<circle cx=\"11\" cy=\"11\" r=\"6.5\"/><path d=\"m16 16 4.5 4.5\"/>";
    readonly home: "<path d=\"M4 10.2 12 4l8 6.2V19a1 1 0 0 1-1 1h-4.5v-5.5h-5V20H5a1 1 0 0 1-1-1z\"/>";
    readonly library: "<rect x=\"4\" y=\"7\" width=\"11\" height=\"13\" rx=\"2\"/><path d=\"M8 4h9a3 3 0 0 1 3 3v10\"/>";
    readonly heart: "<path d=\"M12 19.5s-7.5-4.4-7.5-10A4.2 4.2 0 0 1 12 7a4.2 4.2 0 0 1 7.5 2.5c0 5.6-7.5 10-7.5 10z\"/>";
    readonly plus: "<path d=\"M12 5v14M5 12h14\"/>";
    readonly more: "<circle cx=\"5.5\" cy=\"12\" r=\"1.4\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"12\" cy=\"12\" r=\"1.4\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"18.5\" cy=\"12\" r=\"1.4\" fill=\"currentColor\" stroke=\"none\"/>";
    readonly queue: "<path d=\"M4 6.5h16M4 12h10M4 17.5h7\"/><path d=\"M16 14.5v6l4.5-3z\"/>";
    readonly user: "<circle cx=\"12\" cy=\"8.5\" r=\"3.8\"/><path d=\"M4.5 20c1-3.6 4-5.5 7.5-5.5s6.5 1.9 7.5 5.5\"/>";
    readonly settings: "<path d=\"M5 4v6M5 14v6M12 4v3M12 11v9M19 4v9M19 17v3\"/><circle cx=\"5\" cy=\"12\" r=\"2\"/><circle cx=\"12\" cy=\"9\" r=\"2\"/><circle cx=\"19\" cy=\"15\" r=\"2\"/>";
    readonly chevron: "<path d=\"m9.5 6 6 6-6 6\"/>";
    readonly 'chevron-down': "<path d=\"m6 9.5 6 6 6-6\"/>";
    readonly close: "<path d=\"m6.5 6.5 11 11m0-11-11 11\"/>";
    readonly check: "<path d=\"m5.5 12.5 4 4 9-9\"/>";
    readonly spark: "<path d=\"M12 3.5c.5 4.3 2.2 6 6.5 6.5-4.3.5-6 2.2-6.5 6.5-.5-4.3-2.2-6-6.5-6.5 4.3-.5 6-2.2 6.5-6.5z\"/>";
    readonly trending: "<path d=\"m4 16.5 5-5 3.5 3.5L20 7.5\"/><path d=\"M15 7.5h5v5\"/>";
    readonly alert: "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 8v4.5\"/><circle cx=\"12\" cy=\"16\" r=\".9\" fill=\"currentColor\" stroke=\"none\"/>";
    readonly 'check-circle': "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"m8.5 12.2 2.4 2.4 4.6-4.8\"/>";
    readonly info: "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 11v5\"/><circle cx=\"12\" cy=\"8\" r=\".9\" fill=\"currentColor\" stroke=\"none\"/>";
    readonly clock: "<circle cx=\"12\" cy=\"12\" r=\"8.5\"/><path d=\"M12 7.5V12l3 2\"/>";
    readonly grip: "<circle cx=\"9\" cy=\"7\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"15\" cy=\"7\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"9\" cy=\"12\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"15\" cy=\"12\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"9\" cy=\"17\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/><circle cx=\"15\" cy=\"17\" r=\"1.1\" fill=\"currentColor\" stroke=\"none\"/>";
    readonly menu: "<path d=\"M4 7h16M4 12h16M4 17h16\"/>";
};
export type IconName = keyof typeof ICONS;
export declare const FILLABLE: ReadonlySet<IconName>;

// ---- core ----
export declare const React: any;
export type Size = 'sm' | 'md' | 'lg';
export type ForcedState = 'hover' | 'pressed' | 'focus';
export declare function cx(...parts: Array<string | false | null | undefined>): string;
export declare function useControlled<T>(value: T | undefined, defaultValue: T, onChange?: (v: T) => void): [T, (v: T) => void];
export interface IconProps {
    name: IconName;
    size?: number;
    filled?: boolean;
    label?: string;
    className?: string;
}
export declare function Icon({ name, size, filled, label, className }: IconProps): any;
export interface SpinnerProps {
    size?: 'sm' | 'md' | 'lg';
    tone?: 'neutral' | 'iridescent' | 'inverse';
    label?: string;
}
export declare function Spinner({ size, tone, label }: SpinnerProps): any;
export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'tonal' | 'destructive' | 'iridescent';
export interface ButtonProps {
    variant?: ButtonVariant;
    size?: Size;
    iconStart?: IconName;
    iconEnd?: IconName;
    loading?: boolean;
    disabled?: boolean;
    fullWidth?: boolean;
    forceState?: ForcedState;
    className?: string;
    children?: any;
    [rest: string]: any;
}
export declare function Button({ variant, size, iconStart, iconEnd, loading, disabled, fullWidth, forceState, className, children, ...rest }: ButtonProps): any;
export type IconButtonVariant = 'neutral' | 'active' | 'selected' | 'destructive' | 'play';
export interface IconButtonProps {
    icon: IconName;
    label: string;
    variant?: IconButtonVariant;
    size?: Size;
    pressed?: boolean;
    filled?: boolean;
    disabled?: boolean;
    forceState?: ForcedState;
    className?: string;
    [rest: string]: any;
}
export declare function IconButton({ icon, label, variant, size, pressed, filled, disabled, forceState, className, ...rest }: IconButtonProps): any;
export interface ArtworkProps {
    src?: string;
    art?: number;
    alt?: string;
    shape?: 'square' | 'circle';
    className?: string;
}
export declare function Artwork({ src, art, alt, shape, className }: ArtworkProps): any;
export interface SurfaceProps {
    material?: 'neutral' | 'elevated' | 'collectible';
    padding?: 'none' | 'sm' | 'md' | 'lg';
    className?: string;
    children?: any;
    [rest: string]: any;
}
export declare function Surface({ material, padding, className, children, ...rest }: SurfaceProps): any;
export interface LogoProps {
    variant?: 'lockup' | 'symbol' | 'wordmark' | 'app-icon';
    tone?: 'holo' | 'ink';
    height?: number;
    title?: string;
}
export declare function Logo({ variant, tone, height, title }: LogoProps): any;

// ---- forms ----
export interface InputProps {
    label?: string;
    hint?: string;
    error?: string;
    iconStart?: IconName;
    size?: Size;
    forceState?: 'hover' | 'focus';
    disabled?: boolean;
    id?: string;
    [rest: string]: any;
}
export declare function Input({ label, hint, error, iconStart, size, forceState, disabled, id, ...rest }: InputProps): any;
export interface SearchInputProps {
    value?: string;
    defaultValue?: string;
    onChange?: (v: string) => void;
    placeholder?: string;
    shortcut?: string;
    size?: 'md' | 'lg';
    forceState?: 'hover' | 'focus';
    label?: string;
}
export declare function SearchInput({ value, defaultValue, onChange, placeholder, shortcut, size, forceState, label }: SearchInputProps): any;
export interface TextareaProps {
    label?: string;
    hint?: string;
    error?: string;
    maxLength?: number;
    rows?: number;
    value?: string;
    defaultValue?: string;
    onChange?: (v: string) => void;
    disabled?: boolean;
    placeholder?: string;
    forceState?: 'hover' | 'focus';
}
export declare function Textarea({ label, hint, error, maxLength, rows, value, defaultValue, onChange, disabled, placeholder, forceState }: TextareaProps): any;
export interface CheckboxProps {
    label?: string;
    description?: string;
    checked?: boolean;
    defaultChecked?: boolean;
    indeterminate?: boolean;
    disabled?: boolean;
    onChange?: (v: boolean) => void;
}
export declare function Checkbox({ label, description, checked, defaultChecked, indeterminate, disabled, onChange }: CheckboxProps): any;
export interface RadioOption {
    value: string;
    label: string;
    description?: string;
    disabled?: boolean;
}
export interface RadioGroupProps {
    name: string;
    label?: string;
    options: RadioOption[];
    value?: string;
    defaultValue?: string;
    onChange?: (v: string) => void;
}
export declare function RadioGroup({ name, label, options, value, defaultValue, onChange }: RadioGroupProps): any;
export interface RadioProps {
    name?: string;
    value?: string;
    label?: string;
    description?: string;
    checked?: boolean;
    disabled?: boolean;
    onChange?: () => void;
}
export declare function Radio({ name, value, label, description, checked, disabled, onChange }: RadioProps): any;
export interface SwitchProps {
    label?: string;
    description?: string;
    checked?: boolean;
    defaultChecked?: boolean;
    disabled?: boolean;
    onChange?: (v: boolean) => void;
}
export declare function Switch({ label, description, checked, defaultChecked, disabled, onChange }: SwitchProps): any;
export interface SliderProps {
    value?: number;
    defaultValue?: number;
    min?: number;
    max?: number;
    step?: number;
    buffered?: number;
    tone?: 'neutral' | 'iridescent';
    size?: 'sm' | 'md';
    label: string;
    valueText?: string;
    disabled?: boolean;
    showThumb?: 'always' | 'hover';
    onChange?: (v: number) => void;
}
export declare function Slider({ value, defaultValue, min, max, step, buffered, tone, size, label, valueText, disabled, showThumb, onChange }: SliderProps): any;

// ---- display ----
export interface AvatarProps {
    name: string;
    src?: string;
    size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
    ring?: boolean;
    art?: number;
}
export declare function Avatar({ name, src, size, ring, art }: AvatarProps): any;
export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'featured' | 'explicit' | 'new' | 'trending';
export interface BadgeProps {
    tone?: BadgeTone;
    icon?: IconName;
    children?: any;
}
export declare function Badge({ tone, icon, children }: BadgeProps): any;
export interface TagProps {
    children: any;
    onRemove?: () => void;
}
export declare function Tag({ children, onRemove }: TagProps): any;
export interface ChipProps {
    children: any;
    selected?: boolean;
    defaultSelected?: boolean;
    icon?: IconName;
    disabled?: boolean;
    onChange?: (v: boolean) => void;
}
export declare function Chip({ children, selected, defaultSelected, icon, disabled, onChange }: ChipProps): any;
export interface DividerProps {
    orientation?: 'horizontal' | 'vertical';
    label?: string;
    strength?: 'subtle' | 'default';
}
export declare function Divider({ orientation, label, strength }: DividerProps): any;
export interface SkeletonProps {
    shape?: 'text' | 'rect' | 'circle';
    width?: number | string;
    height?: number | string;
    radius?: 'xs' | 'sm' | 'md' | 'card';
}
export declare function Skeleton({ shape, width, height, radius }: SkeletonProps): any;
export interface ProgressProps {
    value?: number;
    max?: number;
    tone?: 'neutral' | 'iridescent';
    size?: 'sm' | 'md';
    label: string;
    showValue?: boolean;
}
export declare function Progress({ value, max, tone, size, label, showValue }: ProgressProps): any;

// ---- overlays ----
export interface TooltipProps {
    content: string;
    placement?: 'top' | 'bottom';
    open?: boolean;
    shortcut?: string;
    children: any;
}
export declare function Tooltip({ content, placement, open, shortcut, children }: TooltipProps): any;
export interface MenuItem {
    id?: string;
    label?: string;
    icon?: IconName;
    shortcut?: string;
    danger?: boolean;
    disabled?: boolean;
    selected?: boolean;
    divider?: boolean;
    heading?: string;
}
export interface MenuProps {
    items: MenuItem[];
    onSelect?: (item: MenuItem) => void;
    width?: number;
    label?: string;
}
export declare function Menu({ items, onSelect, width, label }: MenuProps): any;
export interface DropdownProps {
    trigger: any;
    items: MenuItem[];
    open?: boolean;
    align?: 'start' | 'end';
    onSelect?: (item: MenuItem) => void;
}
export declare function Dropdown({ trigger, items, open, align, onSelect }: DropdownProps): any;
export interface ContextMenuProps {
    items: MenuItem[];
    children: any;
    defaultPosition?: {
        x: number;
        y: number;
    };
    onSelect?: (item: MenuItem) => void;
}
export declare function ContextMenu({ items, children, defaultPosition, onSelect }: ContextMenuProps): any;
export interface PopoverProps {
    title?: string;
    children: any;
    footer?: any;
    width?: number;
    onClose?: () => void;
}
export declare function Popover({ title, children, footer, width, onClose }: PopoverProps): any;
export interface ModalProps {
    open: boolean;
    title: string;
    description?: string;
    children?: any;
    footer?: any;
    size?: 'sm' | 'md' | 'lg';
    inline?: boolean;
    onClose?: () => void;
}
export declare function Modal({ open, title, description, children, footer, size, inline, onClose }: ModalProps): any;
export interface DrawerProps {
    open: boolean;
    title: string;
    side?: 'right' | 'bottom';
    children?: any;
    footer?: any;
    inline?: boolean;
    onClose?: () => void;
}
export declare function Drawer({ open, title, side, children, footer, inline, onClose }: DrawerProps): any;
export interface ToastProps {
    tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info';
    title: string;
    description?: string;
    action?: {
        label: string;
        onClick?: () => void;
    };
    art?: number;
    onClose?: () => void;
}
export declare function Toast({ tone, title, description, action, onClose }: ToastProps): any;

// ---- nav ----
export interface TabItem {
    id: string;
    label: string;
    count?: number;
    disabled?: boolean;
}
export interface TabsProps {
    items: TabItem[];
    value?: string;
    defaultValue?: string;
    onChange?: (id: string) => void;
    size?: 'md' | 'lg';
    label?: string;
}
export declare function Tabs({ items, value, defaultValue, onChange, size, label }: TabsProps): any;
export interface SegmentOption {
    value: string;
    label?: string;
    icon?: IconName;
}
export interface SegmentedControlProps {
    options: SegmentOption[];
    value?: string;
    defaultValue?: string;
    onChange?: (v: string) => void;
    size?: 'sm' | 'md';
    label?: string;
}
export declare function SegmentedControl({ options, value, defaultValue, onChange, size, label }: SegmentedControlProps): any;
export interface SidebarItemProps {
    icon?: IconName;
    label: string;
    active?: boolean;
    disabled?: boolean;
    count?: number;
    art?: any;
    href?: string;
    forceState?: 'hover';
    onClick?: () => void;
}
export declare function SidebarItem({ icon, label, active, disabled, count, art, href, forceState, onClick }: SidebarItemProps): any;
export interface TopNavItemProps {
    label: string;
    active?: boolean;
    disabled?: boolean;
    href?: string;
    forceState?: 'hover';
    onClick?: () => void;
}
export declare function TopNavItem({ label, active, disabled, href, forceState, onClick }: TopNavItemProps): any;
export interface MobileNavItemProps {
    icon: IconName;
    label: string;
    active?: boolean;
    disabled?: boolean;
    badge?: boolean;
    onClick?: () => void;
}
export declare function MobileNavItem({ icon, label, active, disabled, badge, onClick }: MobileNavItemProps): any;
export interface BreadcrumbProps {
    items: Array<{
        label: string;
        href?: string;
    }>;
}
export declare function Breadcrumb({ items }: BreadcrumbProps): any;

// ---- media ----
export type CardState = 'default' | 'hover' | 'selected' | 'playing' | 'disabled' | 'loading';
export interface MediaCardProps {
    title: string;
    subtitle?: string;
    cover?: string;
    art?: number;
    shape?: 'square' | 'circle';
    kind?: 'album' | 'artist' | 'playlist' | 'media';
    badge?: BadgeTone;
    explicit?: boolean;
    state?: CardState;
    stacked?: boolean;
    width?: number | string;
    onOpen?: () => void;
    onPlay?: () => void;
}
export declare function MediaCard({ title, subtitle, cover, art, shape, kind, badge, explicit, state, stacked, width, onOpen, onPlay }: MediaCardProps): any;
export interface AlbumCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'subtitle'> {
    artist: string;
    year?: number;
}
export declare function AlbumCard({ artist, year, ...rest }: AlbumCardProps): any;
export interface ArtistCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'title' | 'subtitle'> {
    name: string;
    listeners?: string;
}
export declare function ArtistCard({ name, listeners, ...rest }: ArtistCardProps): any;
export interface PlaylistCardProps extends Omit<MediaCardProps, 'kind' | 'shape' | 'subtitle' | 'stacked'> {
    owner?: string;
    trackCount?: number;
}
export declare function PlaylistCard({ owner, trackCount, ...rest }: PlaylistCardProps): any;
export interface FeaturedCardProps {
    title: string;
    artist: string;
    category?: string;
    serial?: string;
    cover?: string;
    art?: number;
    meta?: string[];
    badges?: BadgeTone[];
    state?: 'default' | 'hover' | 'playing' | 'loading';
    width?: number | string;
    onPlay?: () => void;
    onOpen?: () => void;
}
export declare function FeaturedCard({ title, artist, category, serial, cover, art, meta, badges, state, width, onPlay, onOpen }: FeaturedCardProps): any;
export type TrackRowState = 'default' | 'hover' | 'selected' | 'playing' | 'paused' | 'unavailable' | 'loading';
export interface TrackRowProps {
    index: number;
    title: string;
    artist: string;
    album?: string;
    duration: number;
    art?: number;
    cover?: string;
    badge?: BadgeTone;
    explicit?: boolean;
    liked?: boolean;
    state?: TrackRowState;
    showCover?: boolean;
    onPlay?: () => void;
    onLike?: () => void;
    onMore?: () => void;
    onSelect?: () => void;
}
export declare function TrackRow({ index, title, artist, album, duration, art, cover, badge, explicit, liked, state, showCover, onPlay, onLike, onMore, onSelect }: TrackRowProps): any;
export declare function TrackListHeader({ showCover }: {
    showCover?: boolean;
}): any;

// ---- player ----
export declare function formatTime(sec: number): string;
export interface PlaybackStateProps {
    state?: 'playing' | 'paused' | 'buffering';
    size?: 'sm' | 'md';
}
export declare function PlaybackState({ state, size }: PlaybackStateProps): any;
export interface TimeLabelProps {
    seconds: number;
    remaining?: boolean;
    tone?: 'secondary' | 'tertiary';
}
export declare function TimeLabel({ seconds, remaining, tone }: TimeLabelProps): any;
export interface PlayerButtonProps {
    kind: 'play' | 'previous' | 'next' | 'shuffle' | 'repeat';
    playing?: boolean;
    active?: boolean;
    repeatMode?: 'off' | 'all' | 'one';
    size?: 'md' | 'lg';
    disabled?: boolean;
    loading?: boolean;
    onClick?: () => void;
}
export declare function PlayerButton({ kind, playing, active, repeatMode, size, disabled, loading, onClick }: PlayerButtonProps): any;
export interface ProgressBarProps {
    position?: number;
    defaultPosition?: number;
    duration: number;
    buffered?: number;
    showTimes?: boolean;
    disabled?: boolean;
    onSeek?: (sec: number) => void;
}
export declare function ProgressBar({ position, defaultPosition, duration, buffered, showTimes, disabled, onSeek }: ProgressBarProps): any;
export interface VolumeSliderProps {
    value?: number;
    defaultValue?: number;
    muted?: boolean;
    defaultMuted?: boolean;
    onChange?: (v: number) => void;
    onMutedChange?: (m: boolean) => void;
    width?: number;
}
export declare function VolumeSlider({ value, defaultValue, muted, defaultMuted, onChange, onMutedChange, width }: VolumeSliderProps): any;
export interface QueueItemProps {
    title: string;
    artist: string;
    duration: number;
    art?: number;
    cover?: string;
    status?: 'current' | 'next' | 'played';
    draggable?: boolean;
    explicit?: boolean;
    onPlay?: () => void;
    onRemove?: () => void;
}
export declare function QueueItem({ title, artist, duration, art, cover, status, draggable, explicit, onPlay, onRemove }: QueueItemProps): any;

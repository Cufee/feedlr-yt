import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';

// Bundle the lockfile-pinned player locally; playback never depends on a CDN.
const destination = new URL('../assets/vendor/', import.meta.url);
const source = new URL('../node_modules/shaka-player/', import.meta.url);
await mkdir(destination, { recursive: true });
for (const [from, to] of [
    ['dist/shaka-player.ui.js', 'shaka-player.ui.js'],
    ['dist/shaka-player.compiled.js', 'shaka-player.compiled.js'],
    ['LICENSE', 'shaka-player.LICENSE'],
]) {
    await copyFile(new URL(from, source), new URL(to, destination));
}

// Shaka's optional Roboto font is hosted by Google. Use local system fonts so
// loading native controls never makes an unrelated third-party request.
const controls = await readFile(new URL('dist/controls.css', source), 'utf8');
await writeFile(new URL('controls.css', destination), controls
    .replace(/@font-face\s*\{[^}]*\}/g, '')
    .replace(/font-family:Roboto\b/gi, 'font-family:system-ui'));

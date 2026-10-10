import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

// Task loads .env for both this process and the Go server. Derive the published
// port from that same URL so development cannot silently connect elsewhere.
try {
    const action = process.argv[2];
    if (!['start', 'stop'].includes(action)) {
        throw new Error('Usage: node scripts/dev-companion.mjs start|stop (via task).');
    }
    let url;
    if (action === 'start') {
        try {
            url = new URL(process.env.COMPANION_URL);
        } catch {
            throw new Error('Set COMPANION_URL in .env, e.g. http://localhost:18282.');
        }
        if (url.protocol !== 'http:' || !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
            || url.username || url.password || url.search || url.hash
            || !['/', '/companion', '/companion/'].includes(url.pathname)) {
            throw new Error('Local Companion requires an http://localhost, 127.0.0.1, or [::1] URL (optionally /companion).');
        }
        if (!/^[a-zA-Z0-9]{16}$/.test(process.env.COMPANION_SECRET || '')) {
            throw new Error('Set COMPANION_SECRET in .env to 16 random alphanumeric characters (openssl rand -hex 8).');
        }
    }

    const root = fileURLToPath(new URL('../', import.meta.url));
    const args = ['compose', '-f', 'docker-compose.dev.yaml'];
    args.push(...(action === 'start'
        ? ['up', '--detach', '--wait', '--wait-timeout', '90', 'companion']
        : ['stop', 'companion']));
    const result = spawnSync('docker', args, {
        cwd: root,
        stdio: 'inherit',
        env: {
            ...process.env,
            FEEDLR_COMPANION_PORT: url ? url.port || '80' : '18282',
            FEEDLR_COMPANION_BIND: url?.hostname === '[::1]' ? '::1' : '127.0.0.1',
        },
    });
    if (result.error) throw new Error(`Could not run Docker Compose: ${result.error.message}`);
    if (result.status !== 0) {
        throw new Error(`Companion ${action} failed. Check Docker/Podman and the local port; container logs may contain credentials.`);
    }
    if (action === 'start') console.log(`Companion is ready at ${url.origin}.`);
} catch (error) {
    console.error(error.message);
    process.exitCode = 1;
}

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..');
const plugin = JSON.parse(fs.readFileSync(path.join(root, 'src', 'plugin.json'), 'utf8'));

test('plugin metadata identifies a backend datasource', () => {
  assert.equal(plugin.id, 'dubsector-ciscomeraki-datasource');
  assert.equal(plugin.type, 'datasource');
  assert.equal(plugin.backend, true);
  assert.equal(plugin.executable, 'gpx_dubsector_meraki');
});

test('included dashboards exist and contain valid Grafana dashboards', () => {
  const dashboards = plugin.includes.filter(({ type }) => type === 'dashboard');
  assert.ok(dashboards.length > 0);

  for (const dashboard of dashboards) {
    const file = path.join(root, dashboard.path);
    assert.ok(fs.existsSync(file), `${dashboard.path} does not exist`);

    const definition = JSON.parse(fs.readFileSync(file, 'utf8'));
    assert.equal(typeof definition.title, 'string', `${dashboard.path} has no title`);
    assert.ok(Array.isArray(definition.panels), `${dashboard.path} has no panels`);
  }
});

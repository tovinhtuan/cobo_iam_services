import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, '..');
const j = JSON.parse(fs.readFileSync(path.join(root, 'api-v1-implemented-contracts.json'), 'utf8'));
const { paths } = j;

const lines = [];
lines.push('openapi: 3.0.3');
lines.push('info:');
lines.push(`  title: ${JSON.stringify(j.info.title)}`);
lines.push(`  version: ${JSON.stringify(j.info.version)}`);
lines.push('  description: |');
for (const line of (j.info.description || '').split('\n')) {
  lines.push(`    ${line}`);
}
lines.push('  x-oas-source: api-v1-implemented-contracts.json');
lines.push('servers:');
lines.push('  - url: /');
lines.push('components:');
lines.push('  securitySchemes:');
lines.push('    bearerAuth:');
lines.push('      type: http');
lines.push('      scheme: bearer');
lines.push('      bearerFormat: JWT');
lines.push('  schemas:');
lines.push('    ErrorBody:');
lines.push('      type: object');
lines.push('      properties:');
lines.push('        error:');
lines.push('          type: object');
lines.push('          properties:');
lines.push('            code: { type: string }');
lines.push('            message: { type: string }');
lines.push('            details: { type: object, additionalProperties: true }');
lines.push('paths:');

const pathKeys = Object.keys(paths).sort();
for (const p of pathKeys) {
  const entry = paths[p];
  const methods = ['get', 'post', 'put', 'patch', 'delete'].filter((m) => entry[m]);
  if (methods.length === 0) {
    continue;
  }
  lines.push(`  ${JSON.stringify(p)}:`);
  for (const method of methods) {
    const op = entry[method];
    const sum = (op.summary || op.operationId || method).replace(/"/g, "'");
    const auth = op.auth || '';
    lines.push(`    ${method}:`);
    lines.push(`      summary: ${JSON.stringify(sum)}`);
    const opId = `${method}${p.replace(/[/{}\s-]/g, '_')}`;
    lines.push(`      operationId: ${JSON.stringify(opId)}`);
    lines.push('      responses:');
    lines.push('        "200": { description: OK }');
    lines.push('        default:');
    lines.push('          description: Error');
    lines.push('          content:');
    lines.push('            application/json:');
    lines.push('              schema: { $ref: "#/components/schemas/ErrorBody" }');
    if (auth === 'bearer' || (auth && auth.includes('bearer') && auth !== 'none')) {
      lines.push('      security: [ { bearerAuth: [] } ]');
    } else if (auth === 'bearer_pre_company') {
      lines.push('      description: "Bearer: pre-company context token"');
      lines.push('      security: [ { bearerAuth: [] } ]');
    } else {
      lines.push('      security: []');
    }
  }
}

const outDir = path.join(root, 'openapi');
fs.mkdirSync(outDir, { recursive: true });
const out = path.join(outDir, 'v1-iam-snapshot.yaml');
fs.writeFileSync(out, lines.join('\n') + '\n', 'utf8');
console.log('Wrote', out, 'lines', lines.length);

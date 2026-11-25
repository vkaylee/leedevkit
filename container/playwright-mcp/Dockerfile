FROM node:22-bookworm

ARG PLAYWRIGHT_MCP_VERSION=0.0.84
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright

RUN npm install --global @playwright/mcp@${PLAYWRIGHT_MCP_VERSION} \
    && node /usr/local/lib/node_modules/@playwright/mcp/node_modules/playwright/cli.js install --with-deps chrome

ENTRYPOINT ["playwright-mcp"]

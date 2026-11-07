# PromoProdutos
Monitor diário de preços de cooktops de indução em **Go**, com execução no **Codex Cloud**, **GitHub Actions** manual, **GitHub Pages** e alertas pelo **Telegram**.

## Produtos iniciais
- Philco PCT40A — Magazine Luiza
- Dako Supreme — Mercado Livre
- Midea Built — Amazon Brasil

## Configuração
1. No GitHub, abra **Settings > Pages** e selecione **Deploy from a branch**, branch `main`, pasta `/docs`.
2. Configure `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID` como secrets no ambiente Codex Cloud. Para execução manual no Actions, mantenha também os secrets em **Settings > Secrets and variables > Actions** do GitHub. Nunca publique seus valores.
3. Edite `config/products.json` e defina `target_price` (em reais). O valor `0` desativa alertas.
4. Em **Actions > Monitor diário > Run workflow**, inicie uma verificação.
5. Painel: https://beemote-dev.github.io/PromoProdutos/ (depois de ativar Pages).

## Execução local
```bash
go run ./cmd/monitor
```

## Como funciona
A automação **Monitorar preços dos cooktops**, no Codex Cloud, está configurada para executar às 09h17 (America/Sao_Paulo), com início em 08/11/2026. A coleta registra uma amostra por produto e dia em `data/history.json`, copia os dados para `docs/` e publica o histórico no GitHub. O painel lê esses arquivos estáticos. O Actions permanece disponível por execução manual, sem cron diário. Veja [operação e secrets no Cloud](docs/cloud-schedule.md).

## Coleta validada
- Magalu: preço à vista no Pix do produto e vendedor da URL, lido do estado público `__NEXT_DATA__`, com confirmação de moeda BRL, variante e disponibilidade.
- Mercado Livre e Amazon: sem método de preço validado nos testes reais de 07/11/2026; registram indisponível e diagnóstico, sem usar preços de recomendações ou anúncios fixos.
- Cada amostra inclui horário, HTTP, URL final, tamanho e motivo da falha. A rotina de histórico e os alertas do Telegram permanecem ativos.

[Testes reais, preços confirmados, APIs e diagnóstico completo](docs/investigation/2026-11-07.md).

## Testes
```bash
go test -race -cover ./...
go vet ./...
```
Os testes usam respostas locais e Telegram simulado. Os probes reais opcionais estão documentados no relatório; Playwright não é dependência do Actions.

## Limitações
- Apenas métodos comprovados são usados. Bloqueios, CAPTCHA, login, alteração de estrutura ou ausência de preço à vista confirmado resultam em **indisponível**, nunca valor estimado.
- O teste local não garante acesso pela rede do GitHub Actions. Nenhum CAPTCHA ou controle de acesso é contornado.
- APIs autenticadas exigem credenciais e acesso apropriados, além de nova validação real antes de habilitar uma integração.
- Os alertas dependem de metas acima de zero e de credenciais do Telegram configuradas.
- A execução no Cloud depende da disponibilidade do ambiente; a publicação do histórico depende do acesso de escrita ao GitHub.

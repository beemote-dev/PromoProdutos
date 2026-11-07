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

## Limitações
- A coleta lê preços estruturados em JSON-LD. Lojas podem bloquear solicitações automáticas ou não expor preços válidos no HTML. Nesse caso o preço aparecerá como **indisponível**, sem inventar valor.
- Não há bypass de bloqueios ou CAPTCHA; integrações oficiais serão preferíveis quando disponíveis.
- Os alertas dependem de metas acima de zero e de credenciais do Telegram configuradas.
- A execução no Cloud depende da disponibilidade do ambiente; a publicação do histórico depende do acesso de escrita ao GitHub.
- Este é um MVP; coletas reais e alertas ainda precisam ser testados.

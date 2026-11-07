# PromoProdutos
Monitor diário de preços de cooktops de indução em **Go**, com **GitHub Actions**, **GitHub Pages** e alertas pelo **Telegram**.

## Produtos iniciais
- Philco PCT40A — Magazine Luiza
- Dako Supreme — Mercado Livre
- Midea Built — Amazon Brasil

## Configuração
1. No GitHub, abra **Settings > Pages** e selecione **Deploy from a branch**, branch `main`, pasta `/docs`.
2. Em **Settings > Secrets and variables > Actions**, crie `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID`.
3. Edite `config/products.json` e defina `target_price` (em reais). O valor `0` desativa alertas.
4. Em **Actions > Monitor diário > Run workflow**, inicie uma verificação.
5. Painel: https://beemote-dev.github.io/PromoProdutos/ (depois de ativar Pages).

## Execução local
```bash
go run ./cmd/monitor
```

## Como funciona
O workflow executa uma vez por dia, aproximadamente às 09h17 (horário de Brasília), registra uma amostra por produto e dia em `data/history.json`, copia os dados para `docs/` e faz commit da atualização. O painel lê esses arquivos estáticos.

## Limitações
- A coleta lê preços estruturados em JSON-LD. Lojas podem bloquear solicitações automáticas ou não expor preços válidos no HTML. Nesse caso o preço aparecerá como **indisponível**, sem inventar valor.
- Não há bypass de bloqueios ou CAPTCHA; integrações oficiais serão preferíveis quando disponíveis.
- Os alertas dependem de metas acima de zero e de credenciais do Telegram configuradas.
- Agendamentos do GitHub Actions podem atrasar, e push automático depende das permissões da branch.
- Este é um MVP; coletas reais e alertas ainda precisam ser testados.

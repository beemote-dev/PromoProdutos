# Execução diária no Codex Cloud

Em 07/11/2026 o usuário autorizou migrar o agendamento diário do GitHub Actions
para o Codex Cloud. A automação **Monitorar preços dos cooktops** foi criada e
habilitada nesta conversa, vinculada ao ambiente Cloud em `/workspace`.

- Horário: 09h17, fuso `America/Sao_Paulo`.
- Primeira data programada: 08/11/2026.
- O cadastro do agendamento não comprova execução futura nem publicação.
- GitHub Actions: removido somente o evento `schedule`; `workflow_dispatch`,
  testes, secrets, alertas e publicação manual permanecem disponíveis.

## O que a automação faz

A tarefa prepara um checkout limpo do repositório e executa o monitor em Go.
Enquanto o PR #1 estiver aberto, usa `codex/validated-cooktop-prices`; após o merge,
usa `main`. Não faz merge automaticamente. Configuração e histórico são lidos da
versão atual de `main`, preservando metas e amostras anteriores.

Somente extratores comprovados e versionados são permitidos. Bloqueios são
registrados como indisponíveis; não se usa CAPTCHA, evasão ou preço de snippet.
A pesquisa de fontes alternativas nesta conversa não habilita coletores novos.

A tarefa foi instruída a publicar somente `data/history.json`, `docs/history.json`
e `docs/products.json` em `main`, preservando alterações concorrentes, sem force
push, e a confirmar a publicação. O resultado na conversa informa preço à vista,
pagamento, loja, bloqueios, publicação e situação dos alertas. Uma falha não deve
ser apresentada como sucesso.

## Telegram e metas

Os secrets do GitHub não são transferidos automaticamente para o Cloud.
Configure `TELEGRAM_BOT_TOKEN` e `TELEGRAM_CHAT_ID` nos secrets do ambiente Cloud,
com acesso à API do Telegram. Não os informe na conversa nem os coloque no Git.
Na criação da automação, nenhum desses secrets estava disponível no ambiente.

A função de envio e as regras existentes foram preservadas. `target_price: 0`
desativa alertas; as três metas estavam em zero. Sem secrets ou metas positivas,
a coleta pode executar, mas não haverá alerta do Telegram. Não foi enviado um
alerta real de teste. Os resultados também são apresentados pela tarefa na conversa.

## Gerenciar ou reverter

Gerencie a automação por seu título no Codex. Mantenha um único agendador diário
ativo. Para retornar ao Actions, pause a automação e restaure sob `on`:

```yaml
  schedule:
    - cron: '17 12 * * *'
```

O horário do cron é UTC. O workflow manual pode ser executado no GitHub sem
reativar o agendamento diário.

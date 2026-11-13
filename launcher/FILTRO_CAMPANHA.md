# Filtro de campanha — Hyrule Launcher 0.3.0

O sandbox ganhou um filtro de missões principais e sequências da história. Ele é gerado no seu computador a partir dos arquivos já instalados do Savage Wind e ativado apenas no processo do sandbox. A campanha normal e os arquivos originais continuam separados.

**Esta implementação ainda precisa de validação dentro do BOTW real.** A geração dos arquivos e a integração com o launcher foram testadas automaticamente; isso não garante que todo NPC, chefe ou gatilho funcione corretamente com a campanha bloqueada.

## Atualizar o sandbox que já funcionou

1. Feche a recomp e o launcher antigo.
2. Extraia o ZIP 0.3.0 e abra `HyruleLauncher.exe`. Os caminhos salvos são reaproveitados.
3. Na seção Sandbox, deixe **Campanha bloqueada · secundárias e santuários**.
4. Use **Jogar sandbox**. A primeira preparação lê os pacotes, gera a camada de mods e mostra o progresso. Pode demorar; não é uma recompilação do jogo.
5. Na recomp, escolha **Continuar**, carregando seu sandbox existente.

**Não é necessário clicar em Novo + backup.** Esse botão cria outro save base; a atualização do filtro não exige reiniciar seu progresso em Hateno.

Depois da primeira preparação, o launcher reutiliza o filtro enquanto a política e os arquivos detectados continuarem iguais. **Atualizar filtro** força uma nova geração sem recriar o save.

## O que o filtro faz

| Parte | Comportamento |
|---|---|
| Missões `Main` | Retiradas de cada tabela QuestProduct encontrada nos pacotes de inicialização e DLC reconhecidos. Isso também retira suas substituições de agenda e eventos ligados às etapas. |
| Missões `Mini` | Preservadas com seus dados, etapas, condições e recompensas. São as secundárias. |
| Missões `Sub` | Preservadas por padrão. São as missões de santuário. |
| Eventos da campanha | Os fluxos das missões removidas e as famílias de cenas identificadas recebem uma ação comum de espera de um quadro e encerramento normal. |
| Memórias e filmes bloqueados | A informação de evento passa a carregar um fluxo curto, sem o filme ou a timeline original. |
| Gatilhos de mapa | EventTags que chamam fluxos bloqueados são retiradas, junto com ligações que apontavam para essas tags. NPCs e outros atores não são apagados por terem um nome de fluxo nos parâmetros. |
| Recursos | A tabela RSTB é atualizada para os recursos gerados e os pacotes que cresceram. |

Uma chamada bloqueada recebe um recurso válido que termina; o programa não simplesmente apaga os arquivos e deixa o jogo esperando carregá-los. Todos os pontos de entrada conhecidos do fluxo, inclusive auxiliares internos ausentes do EventInfo, são incluídos na substituição.

A classificação padrão bloqueia as famílias narrativas de demos 100–199, 600–699 e 700–799, preservando as exceções documentadas 707 e 719. Também trata as cenas 027, 050 e 051 ligadas ao início da campanha e às vozes das Bestas Divinas. Os nomes completos efetivamente encontrados ficam no relatório; o programa não inventa arquivos para IDs que não encontrou.

## O que permanece

Os eventos comuns de abrir baús, cozinhar, dormir, viajar, usar serviços e entrar/sair de santuários não são bloqueados em massa. As cenas das próprias secundárias, geralmente em outras famílias de demos, continuam presentes. A lógica da Lua de Sangue permanece; uma sequência narrativa chamada por ela pode ser substituída.

O filtro não altera torres ou runas e não marca a história como concluída. A criação de novos sandboxes também deixou de tentar conceder runas por flags. Inventário e habilidades vêm da base escolhida, além do paraglider e da posição preparados pelo sandbox.

As condições originais das secundárias são mantidas. Uma missão que exija câmera, runa, item ou progresso específico não recebe esses requisitos automaticamente. Diálogos comuns ainda podem mencionar a história, pois não foram reescritos. Atividades e recompensas da campanha bloqueada não são convertidas em recompensas de servidor nesta versão.

## Configurar para um futuro servidor

A política fica em `<pasta de dados>/hyrule-sandbox-policy.json`. Feche o jogo antes de editar. Exemplo padrão:

```json
{
  "Schema": 1,
  "KeepShrineQuests": true,
  "BlockStoryCinematics": true,
  "AllowFlows": [],
  "BlockFlows": []
}
```

- `KeepShrineQuests: false`: mantém apenas as secundárias `Mini`, retirando também as missões de santuário `Sub`.
- `BlockStoryCinematics: false`: retira o bloqueio padrão por famílias narrativas; as missões principais e seus próprios fluxos continuam filtrados.
- `AllowFlows`: nomes exatos de fluxos que devem ser preservados, como uma exceção escolhida pelo servidor.
- `BlockFlows`: nomes exatos de outros fluxos a bloquear.

Depois, clique em **Atualizar filtro** ou inicie o sandbox. A política não é um servidor e não sincroniza configurações entre jogadores.

## Diagnóstico e recuperação

O relatório está em `<pasta de dados>/hyrule-sandbox-filter/filter.json`. Ele lista as missões principais removidas, as secundárias e de santuário mantidas, os fluxos bloqueados, os gatilhos de mapa encontrados, os arquivos gerados e suas assinaturas. A camada anterior fica em `hyrule-sandbox-filter-previous` após uma atualização bem-sucedida.

Se a preparação falhar, ela não publica uma camada parcial nem informa que o bloqueio foi aplicado. Um catálogo incompatível, arquivo corrompido ou conflito de DLC é relatado. Os arquivos originais e o filtro anterior são preservados.

Para voltar ao comportamento anterior, selecione **Compatibilidade · manter campanha original** e inicie novamente. Seu sandbox continua usando a mesma pasta de save, com o filtro desligado. **Jogar campanha** também inicia sem essa camada.

Se `launch-flags.txt` tiver uma opção que substitua a pasta de save, controle local, overlay ou parâmetros de teste usados pelo launcher, o sandbox informa o conflito. Remova essa opção nas opções de lançamento do Savage Wind. As opções gráficas normais são preservadas.

**Abrir logs** inclui um resumo do filtro. Para testar esta versão: confira o diário de missões, converse com um NPC de secundária, teste loja/baú/cozinha, entre em um santuário e visite um gatilho de história conhecido. Depois salve e retome. Se houver travamento, use o modo de compatibilidade e guarde o diagnóstico para identificar o evento específico.

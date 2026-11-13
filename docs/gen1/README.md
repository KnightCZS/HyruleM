# Geração 1: mundo e atores nativos (desenvolvimento)

**Estado em 13/11/2026: regras do servidor e contrato de integração implementados e testados; criação de atores nativos ainda não implementada. Esta branch não é uma versão jogável com dois Links nativos.** O launcher e a release estável 0.4.2 continuam com o protótipo visual externo.

O vídeo de 71 segundos enviado pelo usuário mostra o Link local andando em Hateno e abrindo seu inventário. Ele ajuda a confirmar o cenário de teste. Não constitui validação de criação, colisão ou combate de um segundo ator nativo.

## O que está no código

| Pedido | Implementado nesta branch | Falta para funcionar em BOTW |
|---|---|---|
| Dois Links com modelos e animações reais | Separação do ator local e remoto, planos de criação/remoção por mundo | Backend para criar um ator Link remoto, aplicar animação, equipamento e colisão |
| Vida e dano individuais | Estado privado por conexão, dano só na vítima e rejeição de dano ambiental em terceiros | Ler/aplicar vida na thread do jogo e interceptar dano antes de duplicá-lo |
| Inventário próprio | Inventário privado, equipar e descartar apenas itens próprios | Mapear itens nativos e reconciliar a bolsa sem tocar nos itens fora do catálogo |
| Arma coletada fica com quem pegou | Coleta atômica; uma disputa tem um único vencedor; registro repetido não recria o item | Interceptar coleta e remoção de ator, aguardar decisão do servidor |
| Links podem bater | Ataque corpo a corpo com distância, cooldown, dano e durabilidade definidos pelo servidor; PvP opcional | Evento real de acerto, animação, reação, hitbox e fórmula compatível com BOTW |
| Monstro troca para quem está mais perto | Seleção exata por distância 3D dentro do alcance, autoridade única e mudança de geração | Apontar IA para o ator correto e impedir a segunda cópia de simular/causar dano |

Os testes de rede usam duas conexões TCP reais com adaptadores sintéticos identificados como tais. Não executam o jogo. A flag `--allow-simulation` é necessária para aceitá-los; o padrão exige todas as capacidades nativas.

## Regras do primeiro mundo

- Duas conexões, cada uma vinculada ao seu ID de jogador. O pacote de ação não escolhe o dono de outro inventário.
- Somente o estado pessoal do destinatário segue em seu snapshot. Para o outro jogador são públicos nome, pose, equipamento visível e estado vivo/morto.
- Um item de chão tem um ID de mapa estável e global. Coletar remove esse objeto do mundo e cria uma entrada somente na bolsa do vencedor. Descartar volta a criar um objeto compartilhado.
- O primeiro cliente ativo registra objetos do mapa. O ID deve incluir mundo e identidade estável do ator, por exemplo `map:MainField:hash-do-ator`; endereços de memória não são identidades de rede.
- Todo monstro vivo escolhe o jogador elegível mais próximo dentro de `aggro_range`, no mesmo mundo, com pose recente e vida positiva. Se outro ficar estritamente mais perto, troca imediatamente na atualização do servidor. Empates exatos mantêm o alvo atual.
- A simulação da criatura acompanha o jogador escolhido. Cada transferência incrementa `epoch`, invalidando poses e ataques antigos. O cooldown da criatura continua durante a troca.
- Sem alvo elegível, a criatura deixa de ter dono; o futuro backend deve suspender sua IA compartilhada. Mortos, jogadores desconectados e poses com mais de dois segundos não recebem autoridade.
- Atualizações de estado são enviadas a 20 Hz. A lógica não espera uma provocação ou um ataque para selecionar o jogador mais próximo.
- Existe token privado de reconexão, com recuperação em memória por dez minutos após sair. Não é conta, autenticação pública nem persistência após reinício do servidor. Duas instâncias no mesmo PC precisam de tokens e pastas distintos.

## Compilar e testar a base

Na pasta `launcher`, com Go 1.23 ou superior:

```sh
go test -race -count=1 ./internal/...
go vet ./internal/gen1 ./internal/nativebridge ./cmd/hyrulem-gen1-server ./cmd/hyrulem-native-probe
go build -o dist/HyruleM-Gen1-Server.exe ./cmd/hyrulem-gen1-server
go build -o dist/HyruleM-Native-Probe.exe ./cmd/hyrulem-native-probe
```

Em Windows, os executáveis são gerados pelo Actions e publicados como **artefato de desenvolvimento separado**. O servidor Gen1 usa protocolo 2 e, por padrão, porta 5051. O menu visual 0.4.x continua com protocolo 1/porta escolhida pelo usuário e não conecta nesse servidor.

```powershell
.\HyruleM-Gen1-Server.exe --listen 0.0.0.0:5051 --catalog .\catalog.test.json --allow-simulation
```

Esse comando é apenas para desenvolver/testar o protocolo. O catálogo incluído usa nomes sintéticos; não representa as armas/monstros reais. IP de LAN ou Radmin e porta TCP escolhida são aceitos. O servidor não executa BOTW, não contém os arquivos do jogo e ainda não tem cliente nativo jogável.

## Diagnóstico para continuar a integração

1. Abra uma instância pelo multiplayer 0.4.2, escolha Continuar e mantenha a janela renderizando.
2. Execute `HyruleM-Native-Probe.exe --slot 1`. Use `--slot 2` para a outra instância; `--slot 0` para o sandbox normal.
3. Envie o `HyruleM-native-report.json` gerado. Se a porta não aparecer no log, informe `--port N`, usando a porta **de controle local** registrada no `savagewind.log`, e não a porta pública do servidor.

O diagnóstico só lê metadados/hash do executável e de `extracted/exefs/main`, e consulta `version`, `help`, `sites`, `mem regions`, `pos`, `camera`, `hm_caps`. Não faz escrita de memória, não cria itens, não prepara sandbox e não lê saves ou o inventário. O relatório não inclui bytes do jogo nem caminhos completos do computador. Ele pode conter coordenadas e endereços de memória locais.

É esperado que o Savage Wind oficial responda **unknown command** para `hm_caps`: isso identifica a falta do backend. Um relatório não torna o backend disponível.

Para investigar criação de atores e alvo de IA, o arquivo necessário é **somente `extracted/exefs/main`** da instalação utilizada, acompanhado do relatório. Nenhum dump completo é necessário. Esse módulo deve ser enviado por arquivo/canal privado para análise, sem incluí-lo no Git público. Não adivinhar offsets com base em Cemu/Wii U: este runtime usa BOTW Switch 1.9.0.

## Limites atuais

O catálogo implementa armas de corpo a corpo e materiais. A fórmula de dano é simplificada, definida no catálogo, em unidades inteiras; não é a fórmula completa de BOTW. Não há armaduras, escudos, arco, flechas, comida, cura, respawn, efeitos elementais, veículos, física compartilhada, eventos, missões nem sincronização geral do mapa. Não há servidor público seguro ou sistema antitrapaça; use sessões privadas de confiança.

Limites da fundação: dois jogadores, 64 monstros, 64 itens de mundo, 128 entradas de inventário por jogador e 512 identidades de registro por sessão. Falta definir streaming completo, Blood Moon e persistência. Bootstrap deve abortar se algum item gerenciado não tiver definição; itens não gerenciados jamais devem ser apagados para caber no catálogo.

[Contrato do backend e pesquisa verificada](NATIVE_BACKEND.md) · [Validação](VALIDACAO.md)
